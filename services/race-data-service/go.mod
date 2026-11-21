module overdrive/services/race-data-service

go 1.27

require (
	github.com/joho/godotenv v1.5.1
	github.com/shopspring/decimal v1.6.0
	github.com/steebchen/prisma-client-go v0.47.0
	overdrive/shared/apierror v0.0.0
	overdrive/shared/bootstrap v0.0.0
	overdrive/shared/contracts v0.0.0
)

require go.mongodb.org/mongo-driver/v2 v2.10.1 // indirect

replace overdrive/shared/bootstrap => ../../shared/bootstrap

replace overdrive/shared/contracts => ../../shared/contracts

replace overdrive/shared/apierror => ../../shared/apierror
