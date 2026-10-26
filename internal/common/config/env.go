package config

import (
	"os"
	"reflect"
	"strings"
)

type envConfig struct {
	PORT              string `key:"PORT"`
	OPENAI_API_KEY    string `key:"OPENAI_API_KEY"`
	ANTHROPIC_API_KEY string `key:"ANTHROPIC_API_KEY"`
}

var env envConfig

func InitEnv() {
	v := reflect.ValueOf(&env).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("key")
		if tag == "" {
			continue
		}

		value := os.Getenv(tag)

		if v.Field(i).CanSet() {
			v.Field(i).SetString(value)
		}
	}
}

func GetEnv() envConfig {
	return env
}

func SetEnvKeyValue(key, value string) {
	v := reflect.ValueOf(&env).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("key")
		if tag == key && v.Field(i).CanSet() {
			v.Field(i).SetString(value)
			break
		}
	}
}

func EnvOrDefault[T any](value, fallback T) T {
	if s, ok := any(value).(string); ok {
		if strings.TrimSpace(s) != "" {
			return value
		}
		return fallback
	}

	if !reflect.ValueOf(value).IsZero() {
		return value
	}

	return fallback
}
