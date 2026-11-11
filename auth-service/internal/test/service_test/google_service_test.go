package service_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qms/auth-service/internal/apperror"
	"qms/auth-service/internal/model"
	"qms/auth-service/internal/service"
	"qms/auth-service/internal/test/testutil"
)

// googleEnv: Service dengan satu ID token uji "tok" milik budi@gmail.com (sub g-1).
func googleEnv(t *testing.T) *testutil.Env {
	t.Helper()
	env := testutil.NewService(t, false)
	env.Google.Identities["tok"] = model.GoogleIdentity{Subject: "g-1", Email: "budi@gmail.com", Name: "Budi Santoso"}
	return env
}

// linkTokenFor mengambil link token dari galat GOOGLE_LINK_REQUIRED.
func linkTokenFor(t *testing.T, err error) string {
	t.Helper()
	var link *service.LinkRequiredError
	require.ErrorAs(t, err, &link)
	require.NotEmpty(t, link.Token)
	return link.Token
}

// --- Akun baru dan akun yang sudah tertaut ---

func TestLoginWithGoogle_NewEmailCreatesPasswordlessVerifiedAccount(t *testing.T) {
	env := googleEnv(t)

	token, err := env.Svc.LoginWithGoogle(ctx, "tok")

	require.NoError(t, err)
	id, err := env.Svc.Authenticate(ctx, token)
	require.NoError(t, err)
	u := env.Store.User(id)
	assert.Equal(t, "budi@gmail.com", u.Email)
	assert.Equal(t, "Budi Santoso", u.Username)
	assert.Equal(t, "g-1", u.GoogleID.String)
	assert.False(t, u.PasswordHash.Valid, "akun Google-only: tanpa password")
	assert.True(t, u.EmailVerifiedAt.Valid, "Google sudah menjamin emailnya: langsung terverifikasi")
	assert.Empty(t, env.Mailer.Sent(), "tidak ada email verifikasi untuk akun Google")
}

func TestLoginWithGoogle_ExistingGoogleAccountLogsInWithoutDuplicates(t *testing.T) {
	env := googleEnv(t)
	first, err := env.Svc.LoginWithGoogle(ctx, "tok")
	require.NoError(t, err)

	again, err := env.Svc.LoginWithGoogle(ctx, "tok")

	require.NoError(t, err)
	a, _ := env.Svc.Authenticate(ctx, first)
	b, _ := env.Svc.Authenticate(ctx, again)
	assert.Equal(t, a, b, "akun yang sama")
	assert.Len(t, env.Store.Users, 1)
}

func TestLoginWithGoogle_MatchesBySubjectEvenIfEmailChangedAtGoogle(t *testing.T) {
	// FR 4.17: pencocokan lewat google_id, bukan email.
	env := googleEnv(t)
	first, err := env.Svc.LoginWithGoogle(ctx, "tok")
	require.NoError(t, err)
	env.Google.Identities["tok-baru"] = model.GoogleIdentity{Subject: "g-1", Email: "budi.baru@gmail.com", Name: "Budi"}

	again, err := env.Svc.LoginWithGoogle(ctx, "tok-baru")

	require.NoError(t, err)
	a, _ := env.Svc.Authenticate(ctx, first)
	b, _ := env.Svc.Authenticate(ctx, again)
	assert.Equal(t, a, b)
	assert.Len(t, env.Store.Users, 1)
}

func TestLoginWithGoogle_GoogleAccountCanStillBeUsedAlongsidePasswordLoginAfterLinking(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	_, err := env.Svc.LoginWithGoogle(ctx, "tok")
	link := linkTokenFor(t, err)
	_, err = env.Svc.LinkGoogle(ctx, link, "rahasia123")
	require.NoError(t, err)

	viaGoogle, err := env.Svc.LoginWithGoogle(ctx, "tok")
	require.NoError(t, err)
	viaPassword, err := env.Svc.Login(ctx, "budi", "rahasia123")
	require.NoError(t, err)

	a, _ := env.Svc.Authenticate(ctx, viaGoogle)
	b, _ := env.Svc.Authenticate(ctx, viaPassword)
	assert.Equal(t, a, b)
}

// --- Email sudah terdaftar: konfirmasi password, jangan tautkan otomatis ---

func TestLoginWithGoogle_ExistingPasswordAccountRequiresLinkAndIsNotLinkedAutomatically(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")

	token, err := env.Svc.LoginWithGoogle(ctx, "tok")

	assert.ErrorIs(t, err, apperror.ErrGoogleLinkRequired)
	assert.Empty(t, token, "tidak ada token login sebelum password dibuktikan")
	assert.NotEmpty(t, linkTokenFor(t, err))
	assert.False(t, env.Store.User(1).GoogleID.Valid, "akun TIDAK boleh tertaut otomatis (cegah pengambilalihan)")
	assert.Len(t, env.Store.Users, 1, "tidak ada akun kedua")
}

func TestLoginWithGoogle_EmailMatchIgnoresCase(t *testing.T) {
	// Registrasi lama tidak menormalkan email; "Budi@Gmail.com" dan "budi@gmail.com" adalah akun yang sama.
	env := googleEnv(t)
	registered(t, env, "Budi@Gmail.com", "budi", "rahasia123")

	_, err := env.Svc.LoginWithGoogle(ctx, "tok")

	assert.ErrorIs(t, err, apperror.ErrGoogleLinkRequired)
	assert.Len(t, env.Store.Users, 1, "tidak membuat akun ganda dengan email berbeda huruf")
}

func TestLoginWithGoogle_AccountAlreadyLinkedToAnotherGoogleIsConflict(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	env.Google.Identities["tok-lain"] = model.GoogleIdentity{Subject: "g-lain", Email: "x@gmail.com"}
	// tautkan akun itu ke g-lain lebih dulu
	env.Store.Users[0].GoogleID = sql.NullString{String: "g-lain", Valid: true}

	token, err := env.Svc.LoginWithGoogle(ctx, "tok")

	assert.ErrorIs(t, err, apperror.ErrDuplicate)
	assert.Empty(t, token)
}

func TestLoginWithGoogle_PasswordlessAccountWithSameEmailCannotBeLinked(t *testing.T) {
	env := googleEnv(t)
	env.Store.AddUser("budi@gmail.com", "budi", "", true) // tanpa password dan tanpa google_id: tidak ada yang bisa dibuktikan

	_, err := env.Svc.LoginWithGoogle(ctx, "tok")

	assert.ErrorIs(t, err, apperror.ErrDuplicate)
}

// --- Token Google ditolak ---

func TestLoginWithGoogle_VerifierErrorsCreateNothing(t *testing.T) {
	for name, verr := range map[string]error{
		"token tidak sah":          apperror.ErrGoogleInvalidToken,
		"email belum diverifikasi": apperror.ErrGoogleEmailNotVerified,
		"Google tidak terjangkau":  apperror.ErrGoogleUnavailable,
	} {
		env := googleEnv(t)
		env.Google.Err = verr

		token, err := env.Svc.LoginWithGoogle(ctx, "tok")

		assert.ErrorIs(t, err, verr, name)
		assert.Empty(t, token, name)
		assert.Empty(t, env.Store.Users, name+": tidak ada akun yang dibuat")
	}
}

func TestLoginWithGoogle_UnknownTokenIsInvalid(t *testing.T) {
	env := googleEnv(t)

	_, err := env.Svc.LoginWithGoogle(ctx, "token-yang-tidak-dikenal")

	assert.ErrorIs(t, err, apperror.ErrGoogleInvalidToken)
	assert.Empty(t, env.Store.Users)
}

func TestLoginWithGoogle_DisabledWhenNoVerifierConfigured(t *testing.T) {
	env := testutil.NewServiceWithoutGoogle(t)

	_, err := env.Svc.LoginWithGoogle(ctx, "tok")

	assert.False(t, env.Svc.GoogleEnabled())
	assert.ErrorIs(t, err, apperror.ErrGoogleUnavailable)
}

// --- Username dari akun Google ---

func TestLoginWithGoogle_UsernameIsSanitized(t *testing.T) {
	cases := map[string]struct {
		name, email, want string
	}{
		"nama biasa":               {"Budi Santoso", "b@gmail.com", "Budi Santoso"},
		"spasi dirapikan":          {"  Budi    Santoso  ", "b@gmail.com", "Budi Santoso"},
		"tag HTML dibuang":         {"<script>alert(1)</script>Budi", "b@gmail.com", "scriptalert(1)/scriptBudi"},
		"karakter kontrol dibuang": {"Bu\x00di\n", "b@gmail.com", "Budi"},
		"nama kosong pakai email":  {"", "budi.s@gmail.com", "budi.s"},
		"hanya tag pakai email":    {"<>", "ani@gmail.com", "ani"},
		"nama dan email kosong":    {"", "@gmail.com", "user"},
		"emoji dan unicode":        {"Budi 😀 Çelik", "b@gmail.com", "Budi 😀 Çelik"},
	}
	for name, tc := range cases {
		env := testutil.NewService(t, false)
		env.Google.Identities["t"] = model.GoogleIdentity{Subject: "g", Email: tc.email, Name: tc.name}

		_, err := env.Svc.LoginWithGoogle(ctx, "t")

		require.NoError(t, err, name)
		got := env.Store.User(1).Username
		assert.Equal(t, tc.want, got, name)
		assert.NotContains(t, got, "<")
		assert.NotContains(t, got, ">")
	}
}

func TestLoginWithGoogle_UsernameIsCappedAtFiftyRunes(t *testing.T) {
	env := testutil.NewService(t, false)
	env.Google.Identities["t"] = model.GoogleIdentity{Subject: "g", Email: "b@gmail.com", Name: strings.Repeat("é", 80)}

	_, err := env.Svc.LoginWithGoogle(ctx, "t")

	require.NoError(t, err)
	assert.Equal(t, 50, len([]rune(env.Store.User(1).Username)))
}

// --- Menautkan akun dengan password ---

func TestLinkGoogle_CorrectPasswordLinksAccountVerifiesEmailAndLogsIn(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	_, err := env.Svc.LoginWithGoogle(ctx, "tok")
	link := linkTokenFor(t, err)
	require.False(t, env.Store.User(1).EmailVerifiedAt.Valid)

	token, err := env.Svc.LinkGoogle(ctx, link, "rahasia123")

	require.NoError(t, err)
	id, err := env.Svc.Authenticate(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
	u := env.Store.User(1)
	assert.Equal(t, "g-1", u.GoogleID.String)
	assert.True(t, u.EmailVerifiedAt.Valid, "Google menjamin email yang sama")
	assert.True(t, u.PasswordHash.Valid, "password lama tetap berlaku")
}

func TestLinkGoogle_WrongPasswordDoesNotLink(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	_, err := env.Svc.LoginWithGoogle(ctx, "tok")
	link := linkTokenFor(t, err)

	token, err := env.Svc.LinkGoogle(ctx, link, "salah-total")

	assert.ErrorIs(t, err, apperror.ErrNotFound, "balasan sama dengan login gagal (401)")
	assert.Empty(t, token)
	assert.False(t, env.Store.User(1).GoogleID.Valid)
}

func TestLinkGoogle_TokenIsBurnedAfterFiveWrongPasswords(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	_, err := env.Svc.LoginWithGoogle(ctx, "tok")
	link := linkTokenFor(t, err)

	for i := 0; i < 5; i++ {
		_, err := env.Svc.LinkGoogle(ctx, link, "salah")
		require.ErrorIs(t, err, apperror.ErrNotFound, "percobaan %d", i+1)
	}
	// setelah 5 kali salah, password yang BENAR pun tidak diterima dengan token itu
	_, err = env.Svc.LinkGoogle(ctx, link, "rahasia123")

	assert.ErrorIs(t, err, apperror.ErrInvalidLinkToken)
	assert.False(t, env.Store.User(1).GoogleID.Valid)
	// user bisa mengulang dari awal dengan link token baru
	_, err = env.Svc.LoginWithGoogle(ctx, "tok")
	fresh := linkTokenFor(t, err)
	_, err = env.Svc.LinkGoogle(ctx, fresh, "rahasia123")
	assert.NoError(t, err)
}

func TestLinkGoogle_TokenIsSingleUse(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	_, err := env.Svc.LoginWithGoogle(ctx, "tok")
	link := linkTokenFor(t, err)
	_, err = env.Svc.LinkGoogle(ctx, link, "rahasia123")
	require.NoError(t, err)

	_, err = env.Svc.LinkGoogle(ctx, link, "rahasia123")

	assert.ErrorIs(t, err, apperror.ErrInvalidLinkToken, "dipakai ulang ditolak walau passwordnya benar")
}

func TestLinkGoogle_RejectsBadLinkTokens(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	loginToken, err := env.Svc.Login(ctx, "budi", "rahasia123")
	require.NoError(t, err)
	craft := func(c jwt.MapClaims) string {
		s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, c).SignedString(testutil.Key)
		require.NoError(t, err)
		return s
	}
	future := time.Now().Add(5 * time.Minute).Unix()
	_, linkErr := env.Svc.LoginWithGoogle(ctx, "tok")
	valid := linkTokenFor(t, linkErr)

	cases := map[string]string{
		"kosong":                           "",
		"sampah":                           "bukan-jwt",
		"token LOGIN dipakai sebagai link": loginToken,
		"kedaluwarsa":                      craft(jwt.MapClaims{"sub": "1", "gid": "g-1", "purpose": "google_link", "exp": time.Now().Add(-time.Minute).Unix()}),
		"purpose salah":                    craft(jwt.MapClaims{"sub": "1", "gid": "g-1", "purpose": "lain", "exp": future}),
		"tanpa gid":                        craft(jwt.MapClaims{"sub": "1", "purpose": "google_link", "exp": future}),
		"sub bukan angka":                  craft(jwt.MapClaims{"sub": "abc", "gid": "g-1", "purpose": "google_link", "exp": future}),
		"user tidak ada":                   craft(jwt.MapClaims{"sub": "999", "gid": "g-1", "purpose": "google_link", "exp": future}),
		"dirusak":                          valid[:len(valid)-4] + "AAAA",
	}
	for name, raw := range cases {
		token, err := env.Svc.LinkGoogle(ctx, raw, "rahasia123")
		assert.ErrorIs(t, err, apperror.ErrInvalidLinkToken, name)
		assert.Empty(t, token, name)
	}
	assert.False(t, env.Store.User(1).GoogleID.Valid, "tidak ada yang tertaut")
}

func TestLinkGoogle_AccountWithoutPasswordCannotProveAnything(t *testing.T) {
	env := googleEnv(t)
	id := env.Store.AddUser("g@x.com", "g", "", true)
	link, err := env.Issuer.IssueLink(id, "g-9")
	require.NoError(t, err)

	_, err = env.Svc.LinkGoogle(ctx, link, "apa-saja")

	assert.ErrorIs(t, err, apperror.ErrInvalidLinkToken)
}

func TestLinkGoogle_AccountAlreadyLinkedElsewhereIsConflictAndSameGoogleIsIdempotent(t *testing.T) {
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	env.Store.Users[0].GoogleID = sql.NullString{String: "g-lain", Valid: true}
	other, err := env.Issuer.IssueLink(1, "g-1")
	require.NoError(t, err)
	same, err := env.Issuer.IssueLink(1, "g-lain")
	require.NoError(t, err)

	_, err = env.Svc.LinkGoogle(ctx, other, "rahasia123")
	assert.ErrorIs(t, err, apperror.ErrDuplicate)

	token, err := env.Svc.LinkGoogle(ctx, same, "rahasia123")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

// --- Link token BUKAN token login ---

func TestLinkToken_IsNotAcceptedAsLoginToken(t *testing.T) {
	// Link token ditandatangani kunci yang sama dan memuat sub. Kalau diterima sebagai sesi, siapa pun yang
	// punya akun Google dengan email korban mendapat sesi korban TANPA password.
	env := googleEnv(t)
	registered(t, env, "budi@gmail.com", "budi", "rahasia123")
	_, err := env.Svc.LoginWithGoogle(ctx, "tok")
	link := linkTokenFor(t, err)

	_, authErr := env.Svc.Authenticate(ctx, link)
	_, verifyErr := env.Issuer.Verify(link)

	assert.ErrorIs(t, authErr, apperror.ErrUnauthenticated)
	assert.Error(t, verifyErr)
	assert.False(t, env.Store.User(1).GoogleID.Valid)
}

func TestLinkToken_IsShortLivedAndCarriesPurposeAndGoogleID(t *testing.T) {
	env := googleEnv(t)
	link, err := env.Issuer.IssueLink(7, "g-7")
	require.NoError(t, err)

	parsed, err := jwt.Parse(link, func(*jwt.Token) (interface{}, error) { return &testutil.Key.PublicKey, nil })
	require.NoError(t, err)
	c := parsed.Claims.(jwt.MapClaims)
	assert.Equal(t, "google_link", c["purpose"])
	assert.Equal(t, "g-7", c["gid"])
	assert.Equal(t, "7", c["sub"])
	iat, exp := int64(c["iat"].(float64)), int64(c["exp"].(float64))
	assert.Equal(t, 5*time.Minute, time.Duration(exp-iat)*time.Second)

	uid, gid, err := env.Issuer.VerifyLink(link)
	require.NoError(t, err)
	assert.Equal(t, int64(7), uid)
	assert.Equal(t, "g-7", gid)
}

func TestLoginWithGoogle_RaceOnCreateRetriesAndFindsTheWinner(t *testing.T) {
	// Dua login Google pertama untuk akun yang sama serentak: yang kalah menabrak UNIQUE, lalu menemukan akun pemenang.
	env := googleEnv(t)
	store := &racingStore{FakeStore: env.Store}
	svc := service.NewService(store, env.Store, env.Issuer, env.Mailer, service.Options{PublicAppURL: "https://app.example"}).WithGoogle(env.Google)

	token, err := svc.LoginWithGoogle(ctx, "tok")

	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Len(t, env.Store.Users, 1, "hanya satu akun")
}

// racingStore: pada CreateGoogle pertama, akun "pemenang" sudah dibuat pihak lain tepat sebelumnya, sehingga
// CreateGoogle milik kita menabrak UNIQUE (ErrDuplicate).
type racingStore struct {
	*testutil.FakeStore
	raced bool
}

func (r *racingStore) CreateGoogle(c context.Context, email, username, googleID string) (model.User, error) {
	if !r.raced {
		r.raced = true
		if _, err := r.FakeStore.CreateGoogle(c, email, username, googleID); err != nil {
			return model.User{}, err
		}
		return model.User{}, apperror.ErrDuplicate
	}
	return r.FakeStore.CreateGoogle(c, email, username, googleID)
}
