package envconfig

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/spf13/viper"
)

type EnvConfig interface {
	DefaultValues()
	Print()
}

func ReadConfigFrom(path string, envConfig EnvConfig) error {
	if reflect.TypeOf(envConfig).Kind() != reflect.Ptr {
		return fmt.Errorf("envconfig must be a pointer to struct")
	}

	if path == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path = pwd + "/config.env"
	}

	envConfig.DefaultValues()
	defer envConfig.Print()
	v := viper.New()

	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	if err := registerStructKeys(v, envConfig); err != nil {
		return fmt.Errorf("failed to register struct keys: %w", err)
	}

	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		fmt.Println(fmt.Errorf("error reading config file %s, using default", err))
	}

	err := v.Unmarshal(envConfig)
	if err != nil {
		return err
	}

	return nil
}

func registerStructKeys(v *viper.Viper, config interface{}) error {
	return registerKeysRecursive(v, reflect.ValueOf(config), "")
}

func registerKeysRecursive(v *viper.Viper, val reflect.Value, prefix string) error {

	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !field.IsExported() {
			continue
		}
		fieldName := getFieldName(field)
		if fieldName == "-" {
			continue
		}

		var key string
		if prefix != "" {
			key = prefix + "." + fieldName
		} else {
			key = fieldName
		}

		if fieldVal.Kind() == reflect.Struct {
			if err := registerKeysRecursive(v, fieldVal, key); err != nil {
				return err
			}
		} else if fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() && fieldVal.Elem().Kind() == reflect.Struct {
			if err := registerKeysRecursive(v, fieldVal, key); err != nil {
				return err
			}
		} else {
			v.SetDefault(key, fieldVal.Interface())
		}
	}

	return nil
}

func getFieldName(field reflect.StructField) string {
	if tag := field.Tag.Get("env"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return strings.ToLower(field.Name)
}
