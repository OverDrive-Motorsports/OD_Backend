module overdrive/gateway

go 1.22

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	overdrive/shared/apierror v0.0.0
	overdrive/shared/authjwt v0.0.0
)

replace overdrive/shared/apierror => ../shared/apierror

replace overdrive/shared/authjwt => ../shared/authjwt
