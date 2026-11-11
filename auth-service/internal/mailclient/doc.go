// Package mailclient: klien HTTP eksternal untuk API internal mail-service (POST /internal/v1/send dengan header
// X-Service-Token). auth-service memakainya untuk email reset password dan verifikasi email; hasilnya hanya sukses
// atau ErrUnavailable. Dipenuhi sebagai service.Mailer.
//
// Isinya:
//   - client.go: Client, New, dan ErrUnavailable.
package mailclient
