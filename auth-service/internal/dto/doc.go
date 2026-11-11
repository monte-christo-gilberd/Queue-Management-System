// Package dto berisi bentuk body request dan isi respons JSON auth-service. Tag json dan binding di sini adalah
// kontrak dengan frontend, jadi tidak boleh diubah tanpa mengubah klien.
//
// Isinya:
//   - auth_dto.go: RegisterRequest, LoginRequest, AuthResponse, ForgotPasswordRequest, ResetPasswordRequest,
//     VerifyEmailRequest, GoogleLoginRequest, dan LinkGoogleRequest.
package dto
