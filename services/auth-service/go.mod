module overdrive/services/auth-service

go 1.25.0

require overdrive/shared/bootstrap v0.0.0

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	golang.org/x/crypto v0.54.0 // indirect
)

replace overdrive/shared/bootstrap => ../../shared/bootstrap
