package hive

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Table = [][]string

type FileTableError struct {
	Path    string
	Message string
}

func (e FileTableError) Error() string {
	return "hive: table error for " + e.Path + ": " + e.Message
}

type Result[T any, E any] struct {
	ok    bool
	value T
	bad   E
}

func Ok[T any, E any](value T) Result[T, E]  { return Result[T, E]{ok: true, value: value} }
func Err[T any, E any](bad E) Result[T, E]   { return Result[T, E]{ok: false, bad: bad} }

func (r Result[T, E]) IsOk() bool    { return r.ok }
func (r Result[T, E]) IsError() bool { return !r.ok }
func (r Result[T, E]) Ok() T         { return r.value }
func (r Result[T, E]) Err() E        { return r.bad }

func (r Result[T, E]) lessThan(other any) bool {
	o, ok := other.(Result[T, E])
	if !ok {
		return false
	}
	if r.ok != o.ok {
		return !r.ok
	}
	if r.ok {
		return Less(r.value, o.value)
	}
	return Less(r.bad, o.bad)
}

func (r Result[T, E]) String() string {
	if r.ok {
		return "Ok(" + ShowIn(r.value) + ")"
	}
	return "Error(" + ShowIn(r.bad) + ")"
}

type Atom int

var atomNames = []string{"Nil"}

func InitAtoms(names []string) { atomNames = names }

func (a Atom) String() string {
	if int(a) >= 0 && int(a) < len(atomNames) {
		return atomNames[a]
	}
	return "#" + strconv.Itoa(int(a))
}

func AtomToStr(a Atom) string { return strconv.Itoa(int(a)) }

func ToStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case Atom:
		return AtomToStr(x)
	default:
		return Show(v)
	}
}

func Show(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ShowIn(v)
}

func ShowIn(v any) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case error:
		return x.Error()
	case fmt.Stringer:
		return x.String()
	}
	return showParts(reflect.ValueOf(v))
}

func showParts(rv reflect.Value) string {
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		parts := make([]string, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			parts = append(parts, ShowIn(rv.Index(i).Interface()))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case reflect.Struct:
		parts := make([]string, 0, rv.NumField())
		for i := 0; i < rv.NumField(); i++ {
			if !rv.Type().Field(i).IsExported() {
				return fmt.Sprint(rv.Interface())
			}
			parts = append(parts, ShowIn(rv.Field(i).Interface()))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case reflect.Pointer, reflect.Interface:
		if !rv.IsNil() {
			return showParts(rv.Elem())
		}
	}
	if !rv.IsValid() {
		return "<nil>"
	}
	return fmt.Sprint(rv.Interface())
}

func Echo(v any) { fmt.Println(Show(v)) }

func Assert(cond bool) {
	if !cond {
		panic("hive: assertion failed")
	}
}

func Concat[T any](a, b []T) []T {
	out := make([]T, 0, len(a)+len(b))
	out = append(out, a...)
	return append(out, b...)
}

func Cell[T any](v T) *T { return &v }

func CloneVec[T any](s []T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, len(s))
	copy(out, s)
	return out
}

func MapVec[T any, K any](s []T, f func(T) K) []K {
	out := make([]K, len(s))
	for i := range s {
		out[i] = f(s[i])
	}
	return out
}

func CloneVecFn[T any](s []T, clone func(T) T) []T {
	if s == nil {
		return nil
	}
	out := make([]T, len(s))
	for i := range s {
		out[i] = clone(s[i])
	}
	return out
}

func Prepend[T any](v *[]T, value T) {
	out := make([]T, 0, len(*v)+1)
	out = append(out, value)
	*v = append(out, (*v)...)
}

func Drop[T any](v *[]T, low, high int) []T {
	if low < 0 {
		low = 0
	}
	if high >= len(*v) {
		high = len(*v) - 1
	}
	if low > high {
		return []T{}
	}
	taken := make([]T, high-low+1)
	copy(taken, (*v)[low:high+1])
	*v = append((*v)[:low], (*v)[high+1:]...)
	return taken
}

func InRange[T any](i int, v []T) bool { return i >= 0 && i < len(v) }

func InRangeStr(i int, s string) bool {
	return i >= 0 && i < utf8.RuneCountInString(s)
}

func Bytes[T any](v []T) int {
	var zero T
	return len(v) * int(reflect.TypeOf(&zero).Elem().Size())
}

func Eq(a, b any) bool {
	if d, ok := a.(interface{ equalTo(any) bool }); ok {
		return d.equalTo(b)
	}
	return reflect.DeepEqual(a, b)
}

var variantOrder = map[string]int{}

func RegisterVariants(names []string) {
	for i, n := range names {
		variantOrder[n] = i
	}
}

func Less(a, b any) bool {
	if o, ok := a.(ordered); ok {
		return o.lessThan(b)
	}
	return lessValue(reflect.ValueOf(a), reflect.ValueOf(b))
}

type ordered interface{ lessThan(any) bool }

func lessValue(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return false
	}
	if a.Kind() == reflect.Interface {
		return lessInterface(a, b)
	}
	switch a.Kind() {
	case reflect.Bool:
		return !a.Bool() && b.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.Int() < b.Int()
	case reflect.Float32, reflect.Float64:
		return a.Float() < b.Float()
	case reflect.String:
		return a.String() < b.String()
	case reflect.Slice, reflect.Array:
		return lessSequence(a, b)
	case reflect.Struct:
		return lessStruct(a, b)
	}
	return false
}

func lessInterface(a, b reflect.Value) bool {
	if a.IsNil() || b.IsNil() {
		return !a.IsNil() && b.IsNil()
	}
	left, right := a.Elem(), b.Elem()
	li, lok := variantOrder[left.Type().Name()]
	ri, rok := variantOrder[right.Type().Name()]
	if lok && rok && li != ri {
		return li < ri
	}
	return lessValue(left, right)
}

func lessSequence(a, b reflect.Value) bool {
	n := a.Len()
	if b.Len() < n {
		n = b.Len()
	}
	for i := 0; i < n; i++ {
		if lessValue(a.Index(i), b.Index(i)) {
			return true
		}
		if lessValue(b.Index(i), a.Index(i)) {
			return false
		}
	}
	return a.Len() < b.Len()
}

func lessStruct(a, b reflect.Value) bool {
	for i := 0; i < a.NumField(); i++ {
		af, bf := a.Field(i), b.Field(i)
		if !af.CanInterface() {
			continue
		}
		if lessValue(af, bf) {
			return true
		}
		if lessValue(bf, af) {
			return false
		}
	}
	return false
}

func Map[T any, K any](values []T, transform func(T) K) []K {
	out := make([]K, len(values))
	for i, v := range values {
		out[i] = transform(v)
	}
	return out
}

func Filter[T any](values []T, keep func(T) bool) []T {
	out := make([]T, 0, len(values))
	for _, v := range values {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func FilterMap[T any, K any, E any](values []T, transform func(T) Result[K, E]) []K {
	out := make([]K, 0, len(values))
	for _, v := range values {
		if r := transform(v); r.IsOk() {
			out = append(out, r.Ok())
		}
	}
	return out
}

func Sort[T any](values []T) []T {
	out := CloneVec(values)
	sort.SliceStable(out, func(i, j int) bool { return Less(out[i], out[j]) })
	return out
}

func SortBy[T any](values []T, first func(T, T) bool) []T {
	out := CloneVec(values)
	sort.SliceStable(out, func(i, j int) bool { return first(out[i], out[j]) })
	return out
}

func SortInPlace[T any](values []T) {
	sort.SliceStable(values, func(i, j int) bool { return Less(values[i], values[j]) })
}

func SortInPlaceBy[T any](values []T, first func(T, T) bool) {
	sort.SliceStable(values, func(i, j int) bool { return first(values[i], values[j]) })
}

func LenStr(s string) int { return utf8.RuneCountInString(s) }

func Join(v []string, sep string) string { return strings.Join(v, sep) }

func Split(s, sep string) []string { return strings.Split(s, sep) }

func ReplaceFirst(s, from, to string) string {
	return strings.Replace(s, from, to, 1)
}

func ReplaceAll(s, from, to string) string {
	return strings.ReplaceAll(s, from, to)
}

func IndexStr(s string, i int) string {
	n := 0
	for _, r := range s {
		if n == i {
			return string(r)
		}
		n++
	}
	return ""
}

func SliceStr(s string, low, high int) string {
	if high < low {
		return ""
	}
	start := -1
	end := len(s)
	n := 0
	for at := range s {
		if n == low {
			start = at
		}
		if n == high+1 {
			end = at
			break
		}
		n++
	}
	if start < 0 {
		return ""
	}
	return s[start:end]
}

func SliceStrFrom(s string, low int) string {
	n := 0
	for at := range s {
		if n == low {
			return s[at:]
		}
		n++
	}
	return ""
}

func IndexOfStr(s, sub string) Result[int, bool] {
	if s == "" {
		return Err[int, bool](false)
	}
	at := strings.Index(s, sub)
	if at < 0 {
		return Err[int, bool](false)
	}
	return Ok[int, bool](utf8.RuneCountInString(s[:at]))
}

func IndexOfVec[T any](v []T, value T) Result[int, bool] {
	for i := range v {
		if Eq(v[i], value) {
			return Ok[int, bool](i)
		}
	}
	return Err[int, bool](false)
}

func Row(t Table, key string) []string {
	for _, row := range t {
		if len(row) > 0 && row[0] == key {
			return row
		}
	}
	return []string{}
}

func Column(t Table, key string) []string {
	if len(t) == 0 {
		return []string{}
	}
	at := -1
	for i, cell := range t[0] {
		if cell == key {
			at = i
			break
		}
	}
	if at < 0 {
		return []string{}
	}
	out := []string{}
	for _, row := range t {
		if at < len(row) {
			out = append(out, row[at])
		}
	}
	return out
}

func ToTable(v []string, wide int) Table {
	out := Table{}
	if wide < 1 {
		return out
	}
	for at := 0; at < len(v); at += wide {
		end := at + wide
		if end > len(v) {
			end = len(v)
		}
		out = append(out, CloneVec(v[at:end]))
	}
	return out
}

func DivInt(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

func DivFloat(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func ModInt(a, b int) int {
	if b == 0 {
		return 0
	}
	return a % b
}

func ModFloat(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return math.Mod(a, b)
}

func PowInt(a, b int) int {
	if b < 0 {
		return 0
	}
	out := 1
	for i := 0; i < b; i++ {
		out = out * a
	}
	return out
}

func PowFloat(a, b float64) float64 { return math.Pow(a, b) }

type Dict[K comparable, V any] struct {
	order []K
	byKey map[K]V
}

func NewDict[K comparable, V any]() Dict[K, V] {
	return Dict[K, V]{order: []K{}, byKey: map[K]V{}}
}

func (d *Dict[K, V]) Set(key K, value V) {
	if d.byKey == nil {
		d.byKey = map[K]V{}
	}
	if _, seen := d.byKey[key]; !seen {
		d.order = append(d.order, key)
	}
	d.byKey[key] = value
}

func (d Dict[K, V]) Get(key K) Result[V, bool] {
	if v, seen := d.byKey[key]; seen {
		return Ok[V, bool](v)
	}
	return Err[V, bool](false)
}

func (d Dict[K, V]) Has(key K) bool {
	_, seen := d.byKey[key]
	return seen
}

func (d *Dict[K, V]) Delete(key K) {
	if _, seen := d.byKey[key]; !seen {
		return
	}
	delete(d.byKey, key)
	for i, k := range d.order {
		if k == key {
			d.order = append(d.order[:i], d.order[i+1:]...)
			break
		}
	}
}

func (d Dict[K, V]) Keys() []K { return CloneVec(d.order) }

func (d Dict[K, V]) Values() []V {
	out := make([]V, 0, len(d.order))
	for _, k := range d.order {
		out = append(out, d.byKey[k])
	}
	return out
}

func (d Dict[K, V]) Len() int { return len(d.order) }

func (d Dict[K, V]) Clone(clone func(V) V) Dict[K, V] {
	out := NewDict[K, V]()
	for _, k := range d.order {
		out.Set(k, clone(d.byKey[k]))
	}
	return out
}

func Same[T any](v T) T { return v }

func (d Dict[K, V]) equalTo(other any) bool {
	o, ok := other.(Dict[K, V])
	if !ok || len(d.byKey) != len(o.byKey) {
		return false
	}
	for k, v := range d.byKey {
		ov, seen := o.byKey[k]
		if !seen || !Eq(v, ov) {
			return false
		}
	}
	return true
}

func (d Dict[K, V]) String() string {
	parts := make([]string, 0, len(d.order))
	for _, k := range d.order {
		parts = append(parts, ShowIn(k)+": "+ShowIn(d.byKey[k]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func MapFromTable(t Table) Dict[string, string] {
	out := NewDict[string, string]()
	for _, row := range t {
		if len(row) >= 2 {
			out.Set(row[0], row[1])
		}
	}
	return out
}

func MapToTable(d Dict[string, string]) Table {
	out := Table{}
	for _, k := range d.order {
		out = append(out, []string{k, d.byKey[k]})
	}
	return out
}

func DictToGo[K comparable, V any](d Dict[K, V]) map[K]V {
	out := make(map[K]V, len(d.byKey))
	for key, value := range d.byKey {
		out[key] = value
	}
	return out
}

func DictToGoFn[K comparable, V any, W any](d Dict[K, V], conv func(V) W) map[K]W {
	out := make(map[K]W, len(d.byKey))
	for key, value := range d.byKey {
		out[key] = conv(value)
	}
	return out
}

func DictFromGo[K comparable, V any](m map[K]V, less func(a, b K) bool) Dict[K, V] {
	out := Dict[K, V]{order: make([]K, 0, len(m)), byKey: make(map[K]V, len(m))}
	for key, value := range m {
		out.order = append(out.order, key)
		out.byKey[key] = value
	}
	sort.SliceStable(out.order, func(i, j int) bool { return less(out.order[i], out.order[j]) })
	return out
}

func DictFromGoFn[K comparable, V any, W any](m map[K]W, less func(a, b K) bool, conv func(W) V) Dict[K, V] {
	out := Dict[K, V]{order: make([]K, 0, len(m)), byKey: make(map[K]V, len(m))}
	for key, value := range m {
		out.order = append(out.order, key)
		out.byKey[key] = conv(value)
	}
	sort.SliceStable(out.order, func(i, j int) bool { return less(out.order[i], out.order[j]) })
	return out
}

type Task struct {
	done  chan struct{}
	value any
	blown any
	once  sync.Once
}

func Spawn(f func() any) *Task {
	t := &Task{done: make(chan struct{})}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.blown = r
			}
			close(t.done)
		}()
		t.value = f()
	}()
	return t
}

func (t *Task) Await() any {
	<-t.done
	if t.blown != nil {
		panic(t.blown)
	}
	return t.value
}

func AwaitAll[T any](tasks []*Task) []T {
	out := make([]T, 0, len(tasks))
	for _, t := range tasks {
		v, _ := t.Await().(T)
		out = append(out, v)
	}
	return out
}

func AwaitAllVoid(tasks []*Task) {
	for _, t := range tasks {
		t.Await()
	}
}

type TaskTimeoutError struct {
	Waited  int
	Message string
}

func AwaitTimeout[T any](t *Task, ms int) Result[T, TaskTimeoutError] {
	select {
	case <-t.done:
		if t.blown != nil {
			panic(t.blown)
		}
		v, _ := t.value.(T)
		return Ok[T, TaskTimeoutError](v)
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return Err[T, TaskTimeoutError](TaskTimeoutError{Waited: ms, Message: "hive: timed out"})
	}
}

func AwaitAllTimeout[T any](tasks []*Task, ms int) Result[[]T, TaskTimeoutError] {
	deadline := time.After(time.Duration(ms) * time.Millisecond)
	out := make([]T, 0, len(tasks))
	for _, t := range tasks {
		select {
		case <-t.done:
			if t.blown != nil {
				panic(t.blown)
			}
			v, _ := t.value.(T)
			out = append(out, v)
		case <-deadline:
			return Err[[]T, TaskTimeoutError](TaskTimeoutError{Waited: ms, Message: "hive: timed out"})
		}
	}
	return Ok[[]T, TaskTimeoutError](out)
}

func TaskSleep(ms int) {
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

type T interface {
	Errorf(format string, args ...any)
	Helper()
}

func TestAssert(t T, ok bool, source string) {
	t.Helper()
	if !ok {
		t.Errorf("assert %s", source)
	}
}

func TestAssertCmp(t T, ok bool, source, left, right string) {
	t.Helper()
	if !ok {
		t.Errorf("assert %s\n  left:  %s\n  right: %s", source, left, right)
	}
}

func TestRecover(t T) {
	if r := recover(); r != nil {
		t.Errorf("panicked: %v", r)
	}
}
