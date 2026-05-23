module overdrive/services/championship-service

go 1.22

require overdrive/shared/bootstrap v0.0.0

require overdrive/shared/contracts v0.0.0

require github.com/steebchen/prisma-client-go v0.47.0

require github.com/joho/godotenv v1.5.1

require github.com/shopspring/decimal v1.4.0

require go.mongodb.org/mongo-driver/v2 v2.0.1 // indirect

replace overdrive/shared/bootstrap => ../../shared/bootstrap

replace overdrive/shared/contracts => ../../shared/contracts
