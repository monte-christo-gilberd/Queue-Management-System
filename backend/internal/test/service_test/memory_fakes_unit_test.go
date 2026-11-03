package service_test

import (
	"context"
	"database/sql"
	"qms/internal/apperror"
	"qms/internal/model"
	"qms/internal/service"
	"sort"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// Fake in-memory untuk antarmuka repository di paket service.
// Semantik yang ditiru dari spesifikasi (product-overview.md):
//   - satu tiket per (antrian, user) -> Insert kedua untuk pasangan yang sama = ErrDuplicate
//   - nomor tiket naik per antrian mulai dari 1
//   - tiket dilayani mengikuti urutan bergabung (sort_key terkecil lebih dulu)

// memTicketRepo: fake service.TicketRepository.
type memTicketRepo struct {
	mu        sync.Mutex
	nextID    int64
	seq       int64
	tickets   []*model.Ticket
	numbers   map[int64]int
	doneSeq   map[int64]int64
	inserts   int
	completes int

	skipSettings map[int64]model.SkipSettings
	rebalances   int
	recalls      int

	// counters: counter tambahan selain default (ID 1). Kunci = ID counter.
	counters map[int64]model.Counter
}

func newMemTicketRepo() *memTicketRepo {
	return &memTicketRepo{numbers: map[int64]int{}, doneSeq: map[int64]int64{}}
}

func (m *memTicketRepo) MaxWaitingSortKey(_ context.Context, _ *sqlx.Tx, queueID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var max int64
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "waiting" && t.SortKey > max {
			max = t.SortKey
		}
	}
	return max, nil
}

func (m *memTicketRepo) Insert(_ context.Context, _ *sqlx.Tx, queueID, userID, sortKey int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inserts++
	// Mirror index parsial tickets_queue_id_user_id_active_key: hanya tiket aktif yang bentrok.
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.UserID == userID && isActiveStatus(t.Status) {
			return model.Ticket{}, apperror.ErrDuplicate
		}
	}
	m.nextID++
	m.numbers[queueID]++
	t := &model.Ticket{
		ID:           m.nextID,
		QueueID:      queueID,
		TicketNumber: m.numbers[queueID],
		SortKey:      sortKey,
		UserID:       userID,
		Status:       "waiting",
		CreatedAt:    time.Now(),
	}
	m.tickets = append(m.tickets, t)
	return *t, nil
}

func (m *memTicketRepo) waitingInOrder(queueID int64) []*model.Ticket {
	var out []*model.Ticket
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "waiting" {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SortKey < out[j].SortKey })
	return out
}

func (m *memTicketRepo) NextWaiting(_ context.Context, _ *sqlx.Tx, queueID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.waitingInOrder(queueID)
	if len(w) == 0 {
		return model.Ticket{}, apperror.ErrNotFound
	}
	return *w[0], nil
}

// MarkCompleted menurunkan tiket yang sedang dilayani menjadi completed; called_at dan counter dipertahankan.
func (m *memTicketRepo) MarkCompleted(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			m.completes++
			m.seq++
			m.doneSeq[ticketID] = m.seq
			t.Status = "completed"
			t.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
			return nil
		}
	}
	return apperror.ErrNotFound
}

// MarkServing menaikkan tiket waiting menjadi currently_served (called_at baru, recall direset).
func (m *memTicketRepo) MarkServing(_ context.Context, _ *sqlx.Tx, ticketID int64, counterID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.Status = "currently_served"
			t.CalledAt = sql.NullTime{Time: time.Now(), Valid: true}
			t.RecalledAt = sql.NullTime{}
			t.CalledCounterID = counterID
			return nil
		}
	}
	return apperror.ErrNotFound
}

func (m *memTicketRepo) CurrentServing(_ context.Context, _ *sqlx.Tx, queueID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "currently_served" {
			return *t, nil
		}
	}
	return model.Ticket{}, apperror.ErrNotFound
}

func (m *memTicketRepo) MarkRecalled(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.RecalledAt = sql.NullTime{Time: time.Now(), Valid: true}
			m.recalls++
			return nil
		}
	}
	return apperror.ErrNotFound
}

// MarkCancelled membatalkan tiket: status cancelled, cancelled_at terisi, completed_at kosong; called_at tetap.
func (m *memTicketRepo) MarkCancelled(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.Status = "cancelled"
			t.CancelledAt = sql.NullTime{Time: time.Now(), Valid: true}
			t.CompletedAt = sql.NullTime{}
			return nil
		}
	}
	return apperror.ErrNotFound
}

// MarkDeleted: status deleted; skip_count dan called_at tidak berubah (perilaku remove 4.10).
func (m *memTicketRepo) MarkDeleted(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.Status = "deleted"
			t.CompletedAt = sql.NullTime{}
			return nil
		}
	}
	return apperror.ErrNotFound
}

// ServingOfCounter: nomor yang sedang dilayani counter ini di antrian ini; tidak ada: ErrNotFound.
func (m *memTicketRepo) ServingOfCounter(_ context.Context, _ *sqlx.Tx, queueID, counterID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "currently_served" && t.CalledCounterID == counterID {
			return *t, nil
		}
	}
	return model.Ticket{}, apperror.ErrNotFound
}

// ReturnToWaiting: nomor yang sedang dilayani kembali ke waiting dengan sort_key baru; skip_count tidak naik.
func (m *memTicketRepo) ReturnToWaiting(_ context.Context, _ *sqlx.Tx, ticketID, sortKey int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID && t.Status == "currently_served" {
			t.Status = "waiting"
			t.SortKey = sortKey
			t.CalledCounterID = 0
			t.CalledAt = sql.NullTime{}
			t.CompletedAt = sql.NullTime{}
			t.RecalledAt = sql.NullTime{}
			return nil
		}
	}
	return apperror.ErrNotFound
}

// GetForUpdate: tiket milik antrian ini; antrian lain atau id tidak dikenal = ErrNotFound.
func (m *memTicketRepo) GetForUpdate(_ context.Context, _ *sqlx.Tx, queueID, ticketID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID && t.QueueID == queueID {
			return *t, nil
		}
	}
	return model.Ticket{}, apperror.ErrNotFound
}

// backdateCall menggeser waktu pemanggilan ke belakang supaya tes tidak perlu menunggu 3 detik.
func (m *memTicketRepo) backdateCall(ticketID int64, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.CalledAt = sql.NullTime{Time: t.CalledAt.Time.Add(-d), Valid: true}
		}
	}
}

func (m *memTicketRepo) MarkSkipped(_ context.Context, _ *sqlx.Tx, ticketID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.Status = "skipped"
			t.SkipCount++
			t.CompletedAt = sql.NullTime{}
			return nil
		}
	}
	return apperror.ErrNotFound
}

func (m *memTicketRepo) SkipSettings(_ context.Context, _ *sqlx.Tx, queueID int64) (model.SkipSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.skipSettings[queueID]; ok {
		return s, nil
	}
	return model.DefaultSkipSettings(), nil
}

func (m *memTicketRepo) WaitingSortKeys(_ context.Context, _ *sqlx.Tx, queueID int64, limit int) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := []int64{}
	for _, t := range m.waitingInOrder(queueID) {
		keys = append(keys, t.SortKey)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys, nil
}

func (m *memTicketRepo) SortKeyTaken(_ context.Context, _ *sqlx.Tx, queueID, sortKey int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.SortKey == sortKey {
			return true, nil
		}
	}
	return false, nil
}

func (m *memTicketRepo) RequeueSkipped(_ context.Context, _ *sqlx.Tx, ticketID, sortKey int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.ID == ticketID {
			t.Status = "waiting"
			t.SkipCount++
			t.SortKey = sortKey
			t.CompletedAt = sql.NullTime{}
			delete(m.doneSeq, ticketID)
			return nil
		}
	}
	return apperror.ErrNotFound
}

func (m *memTicketRepo) RebalanceWaiting(_ context.Context, _ *sqlx.Tx, queueID, gap int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.waitingInOrder(queueID)
	sort.Slice(w, func(i, j int) bool { return w[i].SortKey < w[j].SortKey })
	var base int64
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.SortKey > base {
			base = t.SortKey
		}
	}
	for i, t := range w {
		t.SortKey = base + int64(i+1)*gap
	}
	m.rebalances++
	return nil
}

func (m *memTicketRepo) GetCounterByQueue(_ context.Context, _ int64) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return model.Counter{ID: 1, QueueID: 1, Name: "Counter 1", IsDefault: true}, nil
}

// GetCounter: counter default (ID 1) dan counter tambahan milik antrian yang sama; selain itu ErrNotFound.
func (m *memTicketRepo) GetCounter(_ context.Context, queueID, counterID int64) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.counters[counterID]; ok && c.QueueID == queueID {
		return c, nil
	}
	if counterID == 1 {
		return model.Counter{ID: 1, QueueID: queueID, Name: "Counter 1", IsDefault: true}, nil
	}
	return model.Counter{}, apperror.ErrNotFound
}

func (m *memTicketRepo) ListByQueue(_ context.Context, queueID int64, statuses []string) ([]model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Ticket{}
	for _, t := range m.tickets {
		if t.QueueID != queueID {
			continue
		}
		if len(statuses) > 0 && !contains(statuses, t.Status) {
			continue
		}
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TicketNumber < out[j].TicketNumber })
	return out, nil
}

// HasTicket: hanya tiket aktif (waiting atau currently_served) yang dihitung; tiket selesai tidak.
func (m *memTicketRepo) HasTicket(_ context.Context, queueID, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.UserID == userID && isActiveStatus(t.Status) {
			return true, nil
		}
	}
	return false, nil
}

// HasAnyTicket: tiket dalam status apa pun, termasuk yang sudah selesai.
func (m *memTicketRepo) HasAnyTicket(_ context.Context, queueID, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

// FindByUser: tiket aktif didahulukan (currently_served, lalu waiting); bila tidak ada, tiket terbaru.
func (m *memTicketRepo) FindByUser(_ context.Context, queueID, userID int64) (model.Ticket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.pickForUser(queueID, userID)
	if !ok {
		return model.Ticket{}, apperror.ErrNotFound
	}
	return t, nil
}

// pickForUser: satu tiket user di queue menurut prioritas FindByUser.
func (m *memTicketRepo) pickForUser(queueID, userID int64) (model.Ticket, bool) {
	var best *model.Ticket
	for _, t := range m.tickets {
		if t.QueueID != queueID || t.UserID != userID {
			continue
		}
		if best == nil || rankStatus(t.Status) < rankStatus(best.Status) ||
			(rankStatus(t.Status) == rankStatus(best.Status) && t.ID > best.ID) {
			best = t
		}
	}
	if best == nil {
		return model.Ticket{}, false
	}
	return *best, true
}

func isActiveStatus(status string) bool {
	return status == "waiting" || status == "currently_served"
}

// rankStatus: urutan prioritas tiket untuk satu user di satu queue (makin kecil makin didahulukan).
func rankStatus(status string) int {
	switch status {
	case "currently_served":
		return 0
	case "waiting":
		return 1
	default:
		return 2
	}
}

// CurrentServedNumber: ticket_number tiket yang sedang dilayani di queue ini.
func (m *memTicketRepo) CurrentServedNumber(_ context.Context, queueID int64) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "currently_served" {
			return t.TicketNumber, true, nil
		}
	}
	return 0, false, nil
}

func (m *memTicketRepo) RecentCompletedNumbers(_ context.Context, queueID int64, n int) ([]int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	nums := m.completedNewestFirst(queueID)
	if len(nums) > n {
		nums = nums[:n]
	}
	return nums, nil
}

// completedNewestFirst: nomor tiket yang sudah selesai, urutan selesai terbaru dulu.
func (m *memTicketRepo) completedNewestFirst(queueID int64) []int {
	type done struct {
		seq    int64
		number int
	}
	var list []done
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "completed" {
			list = append(list, done{seq: m.doneSeq[t.ID], number: t.TicketNumber})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].seq > list[j].seq })
	out := make([]int, 0, len(list))
	for _, d := range list {
		out = append(out, d.number)
	}
	return out
}

func (m *memTicketRepo) FirstWaitingNumber(_ context.Context, queueID int64) (int, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w := m.waitingInOrder(queueID)
	if len(w) == 0 {
		return 0, false, nil
	}
	return w[0].TicketNumber, true, nil
}

func (m *memTicketRepo) CountWaitingBefore(_ context.Context, queueID, sortKey int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, t := range m.tickets {
		if t.QueueID == queueID && t.Status == "waiting" && t.SortKey < sortKey {
			n++
		}
	}
	return n, nil
}

// ListByUser: satu baris per antrian, tiket dipilih dengan prioritas FindByUser.
func (m *memTicketRepo) ListByUser(_ context.Context, userID int64) ([]model.UserTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.UserTicket{}
	seen := map[int64]bool{}
	for _, t := range m.tickets {
		if t.UserID != userID || seen[t.QueueID] {
			continue
		}
		seen[t.QueueID] = true
		picked, _ := m.pickForUser(t.QueueID, userID)
		out = append(out, model.UserTicket{Ticket: picked})
	}
	return out, nil
}

// ticketsOf: salinan semua tiket di antrian tertentu (tanpa urutan khusus).
func (m *memTicketRepo) ticketsOf(queueID int64) []model.Ticket {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []model.Ticket
	for _, t := range m.tickets {
		if t.QueueID == queueID {
			out = append(out, *t)
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// memQueueRepo: fake service.QueueRepository.
type memQueueRepo struct {
	mu       sync.Mutex
	nextID   int64
	order    []int64
	queues   map[int64]model.Queue
	settings map[int64]model.QueueSettings
	counters []model.Counter
	skip     map[int64]model.SkipSettings
}

func newMemQueueRepo() *memQueueRepo {
	return &memQueueRepo{
		queues:   map[int64]model.Queue{},
		settings: map[int64]model.QueueSettings{},
	}
}

func (m *memQueueRepo) CreateNewQueue(_ context.Context, _ *sqlx.Tx, ownerID int64, name, desc string) (model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	q := model.Queue{
		ID:           m.nextID,
		OwnerID:      ownerID,
		QueueName:    name,
		QueueDesc:    sql.NullString{String: desc, Valid: desc != ""},
		TicketPrefix: "A",
		Status:       "active",
		CreatedAt:    time.Now(),
	}
	m.queues[q.ID] = q
	m.order = append(m.order, q.ID)
	// Sama dengan repository nyata: satu baris queue_settings ikut dibuat dalam tx.
	m.settings[q.ID] = model.QueueSettings{QueueID: q.ID, QRType: model.QueueQRTypeStatic}
	return q, nil
}

func (m *memQueueRepo) GetQueueByID(_ context.Context, queueID int64) (model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.queues[queueID]
	if !ok {
		return model.Queue{}, apperror.ErrNotFound
	}
	return q, nil
}

func (m *memQueueRepo) ListQueueByOwner(_ context.Context, ownerID int64, activeOnly bool) ([]model.Queue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.Queue{}
	for _, id := range m.order {
		q := m.queues[id]
		if q.OwnerID != ownerID {
			continue
		}
		if activeOnly && q.Status != "active" {
			continue
		}
		out = append(out, q)
	}
	return out, nil
}

func (m *memQueueRepo) SetTicketPrefix(_ context.Context, queueID int64, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.queues[queueID]
	if !ok {
		return apperror.ErrNotFound
	}
	q.TicketPrefix = prefix
	m.queues[queueID] = q
	return nil
}

func (m *memQueueRepo) UpdateQueueDetails(_ context.Context, queueID int64, name, desc string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.queues[queueID]
	if !ok {
		return apperror.ErrNotFound
	}
	q.QueueName = name
	q.QueueDesc = sql.NullString{String: desc, Valid: desc != ""}
	m.queues[queueID] = q
	return nil
}

func (m *memQueueRepo) CloseQueue(_ context.Context, queueID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	q, ok := m.queues[queueID]
	if !ok {
		return apperror.ErrNotFound
	}
	q.Status = "closed"
	q.ClosedAt = sql.NullTime{Time: time.Now(), Valid: true}
	m.queues[queueID] = q
	return nil
}

func (m *memQueueRepo) CreateDefaultCounter(
	_ context.Context,
	queueID int64,
	name string,
) (model.Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	counter := model.Counter{ID: m.nextID + 1, QueueID: queueID, Name: name, IsDefault: false}
	m.counters = append(m.counters, counter)
	return counter, nil
}

func (m *memQueueRepo) GetCounterByQueue(
	_ context.Context,
	queueID int64,
) (model.Counter, error) {
	return model.Counter{}, apperror.ErrNotFound
}

func (m *memQueueRepo) GetQueueSettings(_ context.Context, queueID int64) (model.QueueSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings, ok := m.settings[queueID]
	if !ok {
		return model.QueueSettings{}, apperror.ErrNotFound
	}
	return settings, nil
}

func (m *memQueueRepo) CreateQueueSettingsIfMissing(_ context.Context, queueID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.queues[queueID]; !ok {
		return apperror.ErrNotFound
	}
	if _, ok := m.settings[queueID]; !ok {
		m.settings[queueID] = model.QueueSettings{QueueID: queueID, QRType: model.QueueQRTypeStatic}
	}
	return nil
}

func (m *memQueueRepo) GetSkipSettings(_ context.Context, queueID int64) (model.SkipSettings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.skip[queueID]; ok {
		return s, nil
	}
	return model.DefaultSkipSettings(), nil
}

func (m *memQueueRepo) UpdateSkipSettings(_ context.Context, queueID int64, s model.SkipSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.skip == nil {
		m.skip = map[int64]model.SkipSettings{}
	}
	m.skip[queueID] = s
	return nil
}

func (m *memQueueRepo) UpdateQRType(_ context.Context, queueID int64, qrType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings, ok := m.settings[queueID]
	if !ok {
		return apperror.ErrNotFound
	}
	settings.QRType = qrType
	m.settings[queueID] = settings
	return nil
}

func (m *memQueueRepo) UpdateQRCurrentCode(_ context.Context, queueID int64, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings, ok := m.settings[queueID]
	if !ok {
		return apperror.ErrNotFound
	}
	settings.QRCurrentCode = sql.NullString{String: token, Valid: token != ""}
	m.settings[queueID] = settings
	return nil
}

// dropSettings mensimulasikan antrian lama yang belum punya baris queue_settings
// (dibuat sebelum fitur QR ada), untuk menguji backfill lazy.
func (m *memQueueRepo) dropSettings(queueID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.settings, queueID)
}

// setQRType menyiapkan mode QR tanpa lewat service (mis. antrian sudah dinamis).
func (m *memQueueRepo) setQRType(queueID int64, qrType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	settings := m.settings[queueID]
	settings.QueueID = queueID
	settings.QRType = qrType
	m.settings[queueID] = settings
}

// currentQRCode membaca token statis yang tersimpan (untuk assertion).
func (m *memQueueRepo) currentQRCode(queueID int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings[queueID].QRCurrentCode.String
}

// qrType membaca mode QR yang tersimpan (untuk assertion).
func (m *memQueueRepo) qrType(queueID int64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings[queueID].QRType
}

// memSubscriptions: fake service.SubscriptionChecker.
type memSubscriptions struct {
	active map[int64]bool
	err    error
}

func (s *memSubscriptions) IsActive(_ context.Context, ownerID int64) (bool, error) {
	return s.active[ownerID], s.err
}

// memUserRepo: fake service.UserRepository.
type memUserRepo struct {
	mu     sync.Mutex
	nextID int64
	users  []model.User
	owners map[int64]bool
}

func newMemUserRepo() *memUserRepo {
	return &memUserRepo{owners: map[int64]bool{}}
}

func (m *memUserRepo) Create(_ context.Context, email, username, passwordHash string) (model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Email == email {
			return model.User{}, apperror.ErrDuplicate
		}
	}
	m.nextID++
	u := model.User{
		ID:           m.nextID,
		Email:        email,
		Username:     username,
		PasswordHash: sql.NullString{String: passwordHash, Valid: true},
		CreatedAt:    time.Now(),
	}
	m.users = append(m.users, u)
	return u, nil
}

func (m *memUserRepo) FindByLogin(_ context.Context, identifier string) ([]model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []model.User{}
	for _, u := range m.users {
		if u.Email == identifier || u.Username == identifier {
			out = append(out, u)
		}
	}
	return out, nil
}

func (m *memUserRepo) FindByID(_ context.Context, id int64) (model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return model.User{}, apperror.ErrNotFound
}

func (m *memUserRepo) IsOwner(_ context.Context, userID int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.owners[userID], nil
}

// recordingNotifier: spy untuk service.EventNotifier.
type notifyCall struct {
	QueueID int64
	Action  string
	Payload interface{}
}

type recordingNotifier struct {
	mu    sync.Mutex
	calls []notifyCall
}

func (r *recordingNotifier) Notify(queueID int64, action string, payload interface{}) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, notifyCall{QueueID: queueID, Action: action, Payload: payload})
}

func (r *recordingNotifier) snapshot() []notifyCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]notifyCall(nil), r.calls...)
}

func (r *recordingNotifier) count(action string) int {
	n := 0
	for _, c := range r.snapshot() {
		if c.Action == action {
			n++
		}
	}
	return n
}

// specEnv merakit service nyata di atas fake: QueueService sebagai QueueGuard untuk TicketService.
type specEnv struct {
	queues  *memQueueRepo
	tickets *memTicketRepo
	subs    *memSubscriptions
	events  *recordingNotifier
	qs      *service.QueueService
	ts      *service.TicketService
	ownerID int64
	queueID int64
}
