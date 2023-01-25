package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LoadFromFile(t *testing.T) {
	tests := map[string]struct {
		filename string
		want     TaskWaitConfig
	}{
		"wait config set to zero": {
			filename: "testdata/waitconfig-zero.toml",
			want:     TaskWaitConfig{100, 6},
		},
		"wait config not set": {
			filename: "testdata/no-waitconfig.toml",
			want:     TaskWaitConfig{100, 6},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadFromFile(test.filename)
			assert.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.Equal(t, test.want, cfg.Fargate.TaskWaitConfig)
		})
	}
}
