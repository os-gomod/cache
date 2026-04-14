package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func ApplyEnv(cfg any) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("config: ApplyEnv requires non-nil pointer to struct, got %T", cfg)
	}

	return walkAndBind(v.Elem(), "")
}

func walkAndBind(v reflect.Value, prefix string) error {
	t := v.Type()
	isRootConfig := prefix == "" && t == reflect.TypeOf(Config{})

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fv := v.Field(i)

		// Handle nested structs
		if fv.Kind() == reflect.Struct {
			nestedPrefix := getEnvPrefix(field, prefix)
			if isRootConfig {
				nestedPrefix = ""
			}
			if err := walkAndBind(fv, nestedPrefix); err != nil {
				return err
			}
			continue
		}

		if fv.Kind() == reflect.Ptr && fv.Type().Elem().Kind() == reflect.Struct {
			if fv.IsNil() {
				fv.Set(reflect.New(fv.Type().Elem()))
			}
			nestedPrefix := getEnvPrefix(field, prefix)
			if isRootConfig {
				nestedPrefix = ""
			}
			if err := walkAndBind(fv.Elem(), nestedPrefix); err != nil {
				return err
			}
			continue
		}

		envKey := getEnvKey(field, prefix)
		if envKey == "" {
			continue
		}

		envVal, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		if err := setFieldFromEnv(fv, envVal, envKey); err != nil {
			return fmt.Errorf("config: failed to set %s=%q: %w", envKey, envVal, err)
		}
	}
	return nil
}

func getEnvPrefix(field reflect.StructField, parentPrefix string) string {
	tag := field.Tag.Get("config")
	if tag == "" {
		tag = strings.ToLower(field.Name)
	}
	name := tagName(tag)

	if parentPrefix == "" {
		return name
	}
	return parentPrefix + "_" + name
}

func getEnvKey(field reflect.StructField, prefix string) string {
	if envTag := field.Tag.Get("env"); envTag != "" {
		return strings.ToUpper(envTag)
	}

	configTag := field.Tag.Get("config")
	if configTag == "" {
		configTag = strings.ToLower(field.Name)
	}
	name := tagName(configTag)

	if prefix == "" {
		return "CACHE_" + strings.ToUpper(name)
	}
	return "CACHE_" + strings.ToUpper(prefix) + "_" + strings.ToUpper(name)
}

func setFieldFromEnv(fv reflect.Value, val, _ string) error {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
		return nil

	case reflect.Int, reflect.Int64:
		if fv.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(val)
			if err != nil {
				return fmt.Errorf("invalid duration %q: %w", val, err)
			}
			fv.SetInt(int64(d))
			return nil
		}
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q: %w", val, err)
		}
		fv.SetInt(n)
		return nil

	case reflect.Uint, reflect.Uint64:
		n, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer %q: %w", val, err)
		}
		fv.SetUint(n)
		return nil

	case reflect.Float64:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return fmt.Errorf("invalid float %q: %w", val, err)
		}
		fv.SetFloat(f)
		return nil

	case reflect.Bool:
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("invalid bool %q: %w", val, err)
		}
		fv.SetBool(b)
		return nil

	default:
		return fmt.Errorf("unsupported type %s for env binding", fv.Kind())
	}
}

// tagName extracts the name portion from a config tag. The tag format is
// config:"name" but may also include other key-value pairs in the future.
// We take only the first comma-separated segment.
func tagName(tag string) string {
	if idx := strings.IndexByte(tag, ','); idx >= 0 {
		return tag[:idx]
	}
	return tag
}
