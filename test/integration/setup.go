package integration

import (
	"NYCU-SDC/core-system-backend/test/testdata/setup"

	"go.uber.org/zap"
)

var resourceManager *setup.ResourceManager

func GetOrInitResource() (*setup.ResourceManager, *zap.Logger, error) {
	logger, err := setup.NewTestLogger()
	if err != nil {
		return nil, nil, err
	}

	if resourceManager != nil {
		logger.Info("resource manager already initialized")
		return resourceManager, logger, nil
	}

	resourceManager, err = setup.NewResourceManager(logger)
	if err != nil {
		return nil, nil, err
	}

	return resourceManager, logger, nil
}
