// Package store: query baca (role notify_ro) dan hapus langganan kedaluwarsa.
package store

import (
	"context"
	"errors"

	"qms/notify-service/internal/message"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PG struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *PG {
	return &PG{pool: pool}
}

func (s *PG) Ticket(ctx context.Context, ticketID int64) (message.Ticket, error) {
	var t message.Ticket
	var counter *string
	// Nama counter hanya diisi bila antrian punya lebih dari satu counter (D1).
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.queue_id, t.ticket_number, t.user_id, t.status, q.queue_name, q.ticket_prefix,
		       CASE WHEN (SELECT count(*) FROM counters c0 WHERE c0.queue_id = t.queue_id) > 1 THEN cc.name END
		FROM tickets t
		JOIN queues q ON q.id = t.queue_id
		LEFT JOIN counters cc ON cc.id = t.called_counter_id
		WHERE t.id = $1`, ticketID).Scan(&t.ID, &t.QueueID, &t.Number, &t.UserID, &t.Status, &t.QueueName, &t.Prefix, &counter)
	if counter != nil {
		t.CounterName = *counter
	}
	return t, err
}

func (s *PG) FirstWaiting(ctx context.Context, queueID int64) (*message.Ticket, error) {
	var t message.Ticket
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.queue_id, t.ticket_number, t.user_id, t.status, q.queue_name, q.ticket_prefix
		FROM tickets t JOIN queues q ON q.id = t.queue_id
		WHERE t.queue_id = $1 AND t.status = 'waiting'
		ORDER BY t.sort_key ASC LIMIT 1`, queueID).Scan(&t.ID, &t.QueueID, &t.Number, &t.UserID, &t.Status, &t.QueueName, &t.Prefix)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *PG) Subscriptions(ctx context.Context, userID int64) ([]message.Subscription, error) {
	rows, err := s.pool.Query(ctx, `SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []message.Subscription
	for rows.Next() {
		var sub message.Subscription
		if err := rows.Scan(&sub.Endpoint, &sub.P256dh, &sub.Auth); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

func (s *PG) DeleteSubscription(ctx context.Context, endpoint string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1`, endpoint)
	return err
}
