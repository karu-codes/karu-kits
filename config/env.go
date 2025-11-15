package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

var (
	durationType       = reflect.TypeOf(time.Duration(0))
	timeType           = reflect.TypeOf(time.Time{})
	matchFirstCap      = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllCap        = regexp.MustCompile("([a-z0-9])([A-Z])")
	repeatedUnderscore = regexp.MustCompile("__+")
)

type fieldMeta struct {
	key          string
	envVar       string
	separator    string
	fieldType    reflect.Type
	defaultValue string
	index        []int
}

func prepareFieldMeta(target any, opt options) ([]fieldMeta, error) {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return nil, fmt.Errorf("config: target must be a non-nil pointer")
	}
	elem := val.Elem()
	if elem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("config: target must point to a struct (got %T)", target)
	}

	var metas []fieldMeta
	collectFieldMeta(elem.Type(), nil, nil, opt, &metas)
	return metas, nil
}

func collectFieldMeta(typ reflect.Type, path []string, indexPrefix []int, opt options, metas *[]fieldMeta) {
	typ = derefType(typ)
	if typ.Kind() != reflect.Struct || typ == timeType {
		return
	}

	for i := 0; i < typ.NumField(); i++ {
		fieldInfo := typ.Field(i)
		if !fieldInfo.IsExported() {
			continue
		}

		baseName := baseFieldName(fieldInfo)
		if baseName == "" {
			continue
		}

		currentPath := withPath(path, baseName)
		fieldType := fieldInfo.Type
		indexPath := appendIndices(indexPrefix, fieldInfo.Index)

		if shouldDescend(fieldType) {
			collectFieldMeta(fieldType, currentPath, indexPath, opt, metas)
			continue
		}

		if !isSupportedLeaf(fieldType) {
			continue
		}

		sep := fieldInfo.Tag.Get("envSeparator")
		if sep == "" {
			sep = opt.sliceSeparator
		}

		meta := fieldMeta{
			key:       strings.Join(currentPath, "."),
			envVar:    buildEnvKey(currentPath, fieldInfo, opt.envPrefix),
			separator: sep,
			fieldType: fieldInfo.Type,
			index:     indexPath,
		}

		if def := fieldInfo.Tag.Get("envDefault"); def != "" {
			meta.defaultValue = def
		}

		*metas = append(*metas, meta)
	}
}

func mergeEnv(k *koanf.Koanf, metas []fieldMeta, opt options) error {
	overrides := make(map[string]any)

	for _, meta := range metas {
		if meta.envVar == "" {
			continue
		}
		raw, ok := opt.envLookup(meta.envVar)
		if !ok {
			continue
		}

		value, err := parseEnvValue(meta, raw)
		if err != nil {
			return fmt.Errorf("config: override %s: %w", meta.envVar, err)
		}
		overrides[meta.key] = value
	}

	if len(overrides) == 0 {
		return nil
	}

	if err := k.Load(confmap.Provider(overrides, "."), nil); err != nil {
		return fmt.Errorf("config: apply env overrides: %w", err)
	}
	return nil
}

func parseEnvValue(meta fieldMeta, raw string) (any, error) {
	holder := reflect.New(meta.fieldType).Elem()
	if err := setFieldValue(holder, raw, meta.separator); err != nil {
		return nil, err
	}
	return holder.Interface(), nil
}

func applyDefaults(target any, metas []fieldMeta) error {
	val := reflect.ValueOf(target)
	if val.Kind() != reflect.Pointer || val.IsNil() {
		return fmt.Errorf("config: target must be a non-nil pointer")
	}
	elem := val.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("config: target must point to a struct (got %T)", target)
	}

	for _, meta := range metas {
		if meta.defaultValue == "" {
			continue
		}

		field := elem.FieldByIndex(meta.index)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		if !field.IsZero() {
			continue
		}

		if err := setFieldValue(field, meta.defaultValue, meta.separator); err != nil {
			return fmt.Errorf("config: apply default for %s: %w", meta.key, err)
		}
	}

	return nil
}

func shouldDescend(t reflect.Type) bool {
	t = derefType(t)
	return t.Kind() == reflect.Struct && t != timeType
}

func isSupportedLeaf(t reflect.Type) bool {
	base := derefType(t)
	switch base.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	case reflect.Slice:
		return base.Elem().Kind() == reflect.String
	case reflect.Struct:
		return base == timeType
	default:
		return false
	}
}

func derefType(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func appendIndices(prefix []int, idx []int) []int {
	out := make([]int, len(prefix)+len(idx))
	copy(out, prefix)
	copy(out[len(prefix):], idx)
	return out
}

func withPath(path []string, elem string) []string {
	if elem == "" {
		if len(path) == 0 {
			return nil
		}
		cp := make([]string, len(path))
		copy(cp, path)
		return cp
	}
	cp := make([]string, len(path)+1)
	copy(cp, path)
	cp[len(path)] = elem
	return cp
}

func baseFieldName(field reflect.StructField) string {
	for _, key := range []string{"mapstructure", "yaml", "json"} {
		if tag := cleanTag(field.Tag.Get(key)); tag != "" {
			return tag
		}
	}
	return field.Name
}

func cleanTag(tag string) string {
	if tag == "" {
		return ""
	}
	if idx := strings.Index(tag, ","); idx >= 0 {
		tag = tag[:idx]
	}
	tag = strings.TrimSpace(tag)
	if tag == "" || tag == "-" {
		return ""
	}
	return tag
}

func buildEnvKey(path []string, field reflect.StructField, prefix string) string {
	envTag := field.Tag.Get("env")
	if envTag == "-" {
		return ""
	}
	if envTag != "" {
		return envTag
	}
	if len(path) == 0 {
		return ""
	}

	parts := make([]string, 0, len(path))
	for _, part := range path {
		if part == "" {
			continue
		}
		segment := toScreamingSnake(part)
		if segment != "" {
			parts = append(parts, segment)
		}
	}

	if len(parts) == 0 {
		return ""
	}

	key := strings.Join(parts, "_")
	if prefix != "" {
		if p := toScreamingSnake(prefix); p != "" {
			key = p + "_" + key
		}
	}

	return key
}

func setFieldValue(value reflect.Value, raw, sliceSep string) error {
	if !value.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return setFieldValue(value.Elem(), raw, sliceSep)
	}

	switch value.Kind() {
	case reflect.String:
		value.SetString(raw)
		return nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		value.SetBool(parsed)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if value.Type() == durationType {
			dur, err := time.ParseDuration(raw)
			if err != nil {
				return err
			}
			value.SetInt(int64(dur))
			return nil
		}
		parsed, err := strconv.ParseInt(raw, 10, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetInt(parsed)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		parsed, err := strconv.ParseUint(raw, 10, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetUint(parsed)
		return nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(raw, value.Type().Bits())
		if err != nil {
			return err
		}
		value.SetFloat(parsed)
		return nil
	case reflect.Slice:
		if value.Type().Elem().Kind() == reflect.String {
			value.Set(reflect.ValueOf(splitAndTrim(raw, sliceSep)))
			return nil
		}
		return fmt.Errorf("unsupported slice type %s", value.Type())
	case reflect.Struct:
		if value.Type() == timeType {
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				return err
			}
			value.Set(reflect.ValueOf(t))
			return nil
		}
	}

	return fmt.Errorf("unsupported type %s", value.Type())
}

func splitAndTrim(input, sep string) []string {
	if sep == "" {
		sep = ","
	}
	parts := strings.Split(input, sep)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		result = append(result, strings.TrimSpace(part))
	}
	return result
}

func toScreamingSnake(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, ".", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = matchFirstCap.ReplaceAllString(s, "${1}_${2}")
	s = matchAllCap.ReplaceAllString(s, "${1}_${2}")
	s = repeatedUnderscore.ReplaceAllString(s, "_")
	return strings.ToUpper(s)
}
