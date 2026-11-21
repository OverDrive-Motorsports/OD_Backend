module overdrive/services/ingestion-service

go 1.27

require (
	overdrive/shared/apierror v0.0.0
	overdrive/shared/bootstrap v0.0.0
	overdrive/shared/contracts v0.0.0
)

replace overdrive/shared/bootstrap => ../../shared/bootstrap

replace overdrive/shared/contracts => ../../shared/contracts

replace overdrive/shared/apierror => ../../shared/apierror
