package srv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptionContextPath(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	path := "testing"

	// Act
	got := OptionContextPath(path)

	// Assert
	assert.Equal(got.name, optionContextPath)
	assert.Equal(got.value, path)
}

func TestOptionEnvName(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	envName := "dev"

	// Act
	got := OptionAppEnv(envName)

	// Assert
	assert.Equal(got.name, optionAppEnv)
	assert.Equal(got.value, envName)
}
