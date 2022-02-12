package redis

import (
	"testing"
	"time"
)

func TestSubmit(t *testing.T) {
	type args struct {
		slug string
	}
	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{"journeymap", "minecraft/mc-mods/journeymap", false},
		{"journeymap", "minecraft/mc-mods/journeymap", false},
		{"journeymap", "minecraft/mc-mods/journeymap", false},
		{"journeymap", "minecraft/mc-mods/journeymap", false},
		{"journeymap", "minecraft/mc-mods/journeymap", false},
		{"journeymap", "minecraft/mc-mods/journeymap", false},
		{"journeymap", "minecraft/mc-mods/journeymap", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Submit(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("Submit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	time.Sleep(5 * time.Second)
}