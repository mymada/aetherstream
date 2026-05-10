package trickplay

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageCompiles(t *testing.T) {
	assert.True(t, true, "trickplay package should compile")
}

func TestNewService(t *testing.T) {
	s := NewService("ffmpeg", "/tmp/trickplay")
	assert.NotNil(t, s)
	assert.Equal(t, "ffmpeg", s.ffmpegPath)
	assert.Equal(t, "/tmp/trickplay", s.outputDir)
}
