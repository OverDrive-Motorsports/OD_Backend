module overdrive/services/user-data-service

go 1.27

require (
	github.com/joho/godotenv v1.5.1
	github.com/shopspring/decimal v1.6.0
	github.com/steebchen/prisma-client-go v0.47.0
	overdrive/shared/apierror v0.0.0
	overdrive/shared/authjwt v0.0.0
	overdrive/shared/bootstrap v0.0.0
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.10.1 // indirect
)

replace overdrive/shared/authjwt => ../../shared/authjwt

replace overdrive/shared/apierror => ../../shared/apierror

replace overdrive/shared/bootstrap => ../../shared/bootstrap
