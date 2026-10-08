package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/service/config"
)

func TestIsProductionAcceptsTheRenderedProdName(t *testing.T) {
	t.Parallel()
	assert.True(t, (&config.CoreConfig{Environment: "prod"}).IsProduction())
}

func TestIsProductionAcceptsProduction(t *testing.T) {
	t.Parallel()
	assert.True(t, (&config.CoreConfig{Environment: "production"}).IsProduction())
}

func TestIsProductionRefusesDev(t *testing.T) {
	t.Parallel()
	assert.False(t, (&config.CoreConfig{Environment: "dev"}).IsProduction())
}
