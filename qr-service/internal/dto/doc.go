// Package dto berisi bentuk body JSON yang masuk dan keluar lewat HTTP. Tag json di sini adalah kontrak
// dengan backend (lihat openapi.yaml), jadi jangan diubah tanpa mengubah kontraknya.
//
// Isinya:
//   - token.go: IssueTokenRequest, TokenResponse, VerifyTokenRequest, VerifyTokenResponse.
//   - render.go: RenderRequest.
//   - error.go: ErrorBody dan ErrorResponse, bentuk respons error semua endpoint.
package dto
