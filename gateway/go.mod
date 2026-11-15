module overdrive/gateway

go 1.22

require (
	overdrive/shared/apierror v0.0.0
	github.com/golang-jwt/jwt/v5 v5.2.1
)

replace overdrive/shared/apierror => ../shared/apierror
