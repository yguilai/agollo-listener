package apollox

import (
	"github.com/apolloconfig/agollo/v4"
	"github.com/pkg/errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apolloconfig/agollo/v4/storage"
)

type (
	Configuration interface {
		Prefix() string
	}

	ConfigListener struct {
		target      Configuration
		cache       map[string]*reflect.Value
		namespaces  []string
		waitTimeout time.Duration
		replaceEnv  bool

		mu sync.RWMutex
	}
)

const (
	keySep              = "."
	defaultWaitTimeout  = 5 * time.Second
	apolloTagKey        = "apollo"
	apolloTagSep        = ","
	apolloDefaultTagKey = "default:"
	defaultNamespace    = "application"
)

var (
	ErrCanNotSet               = errors.New("field can not set")
	ErrTypeNotMatch            = errors.New("field type is not match")
	ErrSliceElemNotConvertible = errors.New("slice element type not convertible")
	ErrWaitInitTimeout         = errors.New("wait init timeout")
	ErrMustStructPtr           = errors.New("config target must be a pointer to struct and not nil")

	configurationType = reflect.TypeFor[Configuration]()
)

// NewConfigListener constructor for ConfigListener
func NewConfigListener(targetConfig Configuration, opts ...Option) (*ConfigListener, error) {
	// check target config type, it must be a struct ptr
	typ := reflect.TypeOf(targetConfig)
	if targetConfig == nil || typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return nil, ErrMustStructPtr
	}

	var options options
	for _, opt := range opts {
		opt(&options)
	}

	if len(options.namespaces) == 0 {
		options.namespaces = append(options.namespaces, defaultNamespace)
	}
	if options.waitTimeout <= 0 {
		options.waitTimeout = defaultWaitTimeout
	}

	l := &ConfigListener{
		target:      targetConfig,
		namespaces:  options.namespaces,
		waitTimeout: options.waitTimeout,
		cache:       make(map[string]*reflect.Value),
	}
	rv := reflect.ValueOf(targetConfig).Elem()
	err := l.generateReflectValuesCache(targetConfig.Prefix(), &rv)
	if err != nil {
		return nil, err
	}
	return l, err
}

// OnChange implement storage.ChangeListener of agollo
func (l *ConfigListener) OnChange(event *storage.ChangeEvent) {
	defer func() {
		if err := recover(); err != nil {
			getLogger().Errorf("panic recovered: %v", err)
		}
	}()

	l.mu.Lock()
	defer l.mu.Unlock()

	// ignore an event when its namespace is not in DefaultApolloListener.namespaces
	if !slices.Contains(l.namespaces, event.Namespace) {
		return
	}

	for key, change := range event.Changes {
		if rv, ok := l.cache[key]; ok {
			if change.ChangeType == storage.DELETED {
				//  set a zero value when the change type is storage.DELETED
				rv.Set(reflect.New(rv.Type().Elem()))
				continue
			}

			err := l.mappingFieldValue(key, rv, change.NewValue)
			if err != nil {
				getLogger().Errorf("apollo config change error: %v", err)
				continue
			}
		}
	}
}

// OnNewestChange implement storage.ChangeListener of agollo
func (l *ConfigListener) OnNewestChange(event *storage.FullChangeEvent) {
}

func (l *ConfigListener) Poll(client agollo.Client) {
	routineGroup := NewRoutineGroup()
	for _, namespace := range l.namespaces {
		routineGroup.Run(func() {
			err := l.pollNamespace(client, namespace)
			if err != nil {
				getLogger().Errorf("apollo config polling error, namespace: %s, %v", namespace, err)
			}
		})
	}
	routineGroup.Wait()
}

func (l *ConfigListener) pollNamespace(client agollo.Client, namespace string) error {
	config := client.GetConfig(namespace)
	if !config.GetIsInit() {
		timeout := WaitWithTimeout(config.GetWaitInit(), l.waitTimeout)
		if !timeout {
			return ErrWaitInitTimeout
		}
	}

	config.GetCache().Range(func(key, value interface{}) bool {
		keyString := key.(string)
		if rv, ok := l.cache[keyString]; ok && rv != nil {
			err := l.mappingFieldValue(keyString, rv, value)
			if err != nil {
				getLogger().Errorf("apollo range config error: %v", err)
				return false
			}
		}
		return true
	})
	return nil
}

// contains only for test
func (l *ConfigListener) contains(key string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if _, ok := l.cache[key]; ok {
		return true
	}
	return false
}

func (l *ConfigListener) generateReflectValuesCache(prefix string, rv *reflect.Value) error {
	typ := rv.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldRV := rv.Field(i)

		var key string
		if tag, ok := field.Tag.Lookup(apolloTagKey); ok {
			tagVals := strings.Split(tag, apolloTagSep)
			if len(tagVals) > 0 {
				key = tagVals[0]
			}
			if len(tagVals) > 1 {
				err := l.mappingDefaultValue(tagVals[1], &fieldRV)
				if err != nil {
					return err
				}
			}
		} else {
			key = lowerFirst(field.Name)
		}

		if fieldRV.Kind() == reflect.Ptr {
			if fieldRV.IsNil() {
				fieldRV.Set(reflect.New(fieldRV.Type().Elem()))
			}
			fieldRV = fieldRV.Elem()
		}

		realKey := prefix + keySep + key
		l.cache[realKey] = &fieldRV
		lowerKey := strings.ToLower(realKey)
		if lowerKey != realKey {
			l.cache[lowerKey] = &fieldRV
		}

		if fieldRV.Kind() == reflect.Struct {
			err := l.generateReflectValuesCache(realKey, &fieldRV)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *ConfigListener) mappingFieldValue(key string, field *reflect.Value, value interface{}) error {
	if !field.CanSet() {
		return errors.WithMessagef(ErrCanNotSet, "key: %s", key)
	}

	fieldType := field.Type()
	rvValue := reflect.ValueOf(value)

	if rvValue.Kind() != field.Kind() {
		return errors.WithMessagef(ErrTypeNotMatch, "field type: %s，value type: %T", fieldType, value)
	}

	if !rvValue.Type().ConvertibleTo(fieldType) {
		if rvValue.Kind() == reflect.Slice {
			return l.mappingSliceFieldValue(key, field, &rvValue)
		} else {
			return errors.WithMessagef(ErrTypeNotMatch, "field type: %s，value type: %T", fieldType, value)
		}
	}

	convertedValue := rvValue.Convert(fieldType)
	if l.replaceEnv && convertedValue.Kind() == reflect.String {
		convertedValue.SetString(replaceEnvVar(convertedValue.String()))
	}
	field.Set(convertedValue)
	return nil
}

func (l *ConfigListener) mappingSliceFieldValue(key string, field, rvValue *reflect.Value) error {
	// get slice element type
	sliceType := field.Type()
	elemType := sliceType.Elem()

	// compatibility with pointer types
	isPtr := elemType.Kind() == reflect.Ptr
	if isPtr {
		elemType = elemType.Elem()
	}

	newSlice := reflect.MakeSlice(sliceType, 0, rvValue.Len())

	for i := 0; i < rvValue.Len(); i++ {
		elem := rvValue.Index(i)

		// if elem type is reflect.Interface, need to get actual type
		elemActualType := elem.Type()
		if elemActualType.Kind() == reflect.Interface {
			if !elem.IsNil() {
				elem = elem.Elem()
				elemActualType = elem.Type()
			}
		}

		var finalElem reflect.Value
		if elemActualType != elemType {
			if elemActualType.Kind() == reflect.Map && elemType.Kind() == reflect.Struct {
				// agollo returns a map when the slice element is key-value mode
				finalElem = *(l.convertMapToStruct(elemType, &elem))
			} else if !elemActualType.ConvertibleTo(elemType) {
				return errors.WithMessagef(ErrSliceElemNotConvertible, "key: %s, field: %s, expected: %v, got: %v",
					key, elemType.Name(), elemType, elemActualType)
			} else {
				finalElem = elem.Convert(elemType)
			}
		} else {
			finalElem = elem
		}

		// wrap ptr
		if isPtr {
			if finalElem.Kind() != reflect.Ptr {
				ptr := reflect.New(elemType)
				ptr.Elem().Set(finalElem)
				finalElem = ptr
			}
		}

		newSlice = reflect.Append(newSlice, finalElem)
	}

	field.Set(newSlice)
	return nil
}

func (l *ConfigListener) convertMapToStruct(elemType reflect.Type, elem *reflect.Value) *reflect.Value {
	newStruct := reflect.New(elemType).Elem()
	mapValue := elem.Interface().(map[string]interface{})

	for i := 0; i < elemType.NumField(); i++ {
		field := elemType.Field(i)
		fieldValue := newStruct.Field(i)

		if !fieldValue.CanSet() {
			continue
		}

		mapKey := lowerFirst(field.Name)
		if tag, ok := field.Tag.Lookup(apolloTagKey); ok {
			if tagParts := strings.Split(tag, ","); len(tagParts) > 0 && tagParts[0] != "" {
				mapKey = tagParts[0]
			}
		}

		if val, exists := mapValue[mapKey]; exists && val != nil {
			rv := reflect.ValueOf(val)
			if rv.Type().ConvertibleTo(fieldValue.Type()) {
				fieldValue.Set(rv.Convert(fieldValue.Type()))
			}
		}
	}
	return &newStruct
}

func (l *ConfigListener) mappingDefaultValue(tagDefaultValue string, fieldRV *reflect.Value) error {
	defaultVal := strings.TrimSpace(tagDefaultValue)
	if strings.Index(defaultVal, apolloDefaultTagKey) != -1 && fieldRV.CanSet() {
		defaultVal = strings.TrimPrefix(defaultVal, apolloDefaultTagKey)
		defaultVal = strings.TrimLeft(defaultVal, "'")
		defaultVal = strings.TrimRight(defaultVal, "'")
		err := l.convertStringValue(fieldRV, defaultVal)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *ConfigListener) convertStringValue(fieldVal *reflect.Value, newValue string) error {
	switch fieldVal.Kind() {
	case reflect.String:
		if l.replaceEnv {
			newValue = replaceEnvVar(newValue)
		}
		fieldVal.SetString(newValue)
	case reflect.Bool:
		boolVal, err := strconv.ParseBool(newValue)
		if err != nil {
			return err
		}
		fieldVal.SetBool(boolVal)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(newValue, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.SetInt(intVal)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(newValue, 10, 64)
		if err != nil {
			return err
		}
		fieldVal.SetUint(uintVal)
	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(newValue, 64)
		if err != nil {
			return err
		}
		fieldVal.SetFloat(floatVal)
	default:
		getLogger().Errorf("unsupported type: %s", fieldVal.Kind())
	}
	return nil
}

// RegisterByParentConfig register listeners for those field of parentConfig that implements Configuration
func RegisterByParentConfig(client agollo.Client, parentConfig interface{}, opts ...Option) error {
	typ := reflect.TypeOf(parentConfig)
	if parentConfig == nil || typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return ErrMustStructPtr
	}

	listeners := make([]*ConfigListener, 0)
	rv := reflect.ValueOf(parentConfig).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		elemType := rt.Field(i)
		elemValue := rv.Field(i)
		if elemType.Type.Implements(configurationType) {
			targetConfig := elemValue.Interface().(Configuration)
			listener, err := NewConfigListener(targetConfig, opts...)
			if err != nil {
				return err
			}
			client.AddChangeListener(listener)
			listeners = append(listeners, listener)
		}
	}

	routineGroup := NewRoutineGroup()
	for _, listener := range listeners {
		routineGroup.RunSafe(func() {
			listener.Poll(client)
		})
	}
	routineGroup.Wait()
	return nil
}
