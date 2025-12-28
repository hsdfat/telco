package envconfig

import (
	"reflect"
	"sort"
)

type Logger interface {
	Infof(format string, args ...interface{})
	Infow(msg string, keysAndValues ...interface{})
}

func Show(logger Logger, config interface{}) {
	configMap := convertToMap(config)
	if configMap == nil {
		logger.Infof("No configuration to show")
		return
	}
	logger.Infof("----------Env Value----------")
	keys := make([]string, 0, len(configMap))
	for k := range configMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		logger.Infof("\t%s= %v", k, configMap[k])
	}
	logger.Infof("------------------------------")
}

func convertToMap(config interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(config)
	typ := reflect.TypeOf(config)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
		typ = typ.Elem()
	}
	if val.Kind() != reflect.Struct {
		return nil
	}
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		if !field.CanInterface() || !fieldType.IsExported() {
			continue
		}
		fieldName := fieldType.Name
		if tag := fieldType.Tag.Get("env"); tag != "" && tag != "-" {
			fieldName = tag
		}
		result[fieldName] = field.Interface()
	}
	return result
}