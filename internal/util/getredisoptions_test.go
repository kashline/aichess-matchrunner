package util

import (
	"reflect"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestGetRedisOptions(t *testing.T) {
	result := GetRedisOptions()
	expected := &redis.Options{
		Addr: "host.docker.internal:6379",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetRedisOptions() = %+v; want %+v", result, expected)
	}
}
