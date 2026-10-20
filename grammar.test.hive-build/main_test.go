package main

import (
	"testing"

	"hiveapp/hive"
)

func TestHive(__t *testing.T) {
	hive.InitAtoms([]string{"Nil"})
	__t.Run("every slide a rule is drawn on is in the tour", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var all []slides_8_Slide = slides_8_all()
		_ = all
		for _, entry := range grammar_5_slideRules() {
			_ = entry
			var found hive.Result[int, string] = main_11_slideIndex(all, entry.Title)
			_ = found
			{
				_m1 := false
				if _t2 := found; _t2.IsError() {
					why := _t2.Err()
					_ = why
					_m1 = true
					hive.Echo(why)
				}
				_ = _m1
			}
			hive.TestAssert(__t, (found).IsOk(), "found is Result.Ok(_)")
		}
	})
	__t.Run("every rule is drawn on a canvas with something on it", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var drawn []string = []string{}
		_ = drawn
		for _, entry := range grammar_5_slideRules() {
			_ = entry
			for _, r := range entry.Rules {
				_ = r
				drawn = append(drawn, hive.UiHtml(rail_4_view(r.Rows)))
			}
		}
		for _, html := range drawn {
			_ = html
			hive.TestAssert(__t, (hive.IndexOfStr(html, "<svg")).IsOk(), "indexOf(html, \"<svg\") is Result.Ok(_)")
			hive.TestAssert(__t, (hive.IndexOfStr(html, "<polyline")).IsOk(), "indexOf(html, \"<polyline\") is Result.Ok(_)")
		}
	})
	__t.Run("every diagram says what the EBNF under it says", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		for _, entry := range grammar_5_slideRules() {
			_ = entry
			for _, r := range entry.Rules {
				_ = r
				var back string = readBack(r)
				_ = back
				if !((back == r.Ebnf)) {
					hive.Echo(r.Name + " is drawn as\n  " + back + "\nbut printed as\n  " + r.Ebnf)
				}
				_l3 := back
				_r4 := r.Ebnf
				hive.TestAssertCmp(__t, (_l3 == _r4), "back == r.ebnf", hive.Show(_l3), hive.Show(_r4))
			}
		}
	})
	__t.Run("a slide with no rule draws none", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l5 := len(grammar_5_forSlide("Strings"))
		_r6 := 0
		hive.TestAssertCmp(__t, (_l5 == _r6), "len(grammar_5_forSlide(\"Strings\")) == 0", hive.Show(_l5), hive.Show(_r6))
		_l7 := len(grammar_5_forSlide("Loops"))
		_r8 := 2
		hive.TestAssertCmp(__t, (_l7 == _r8), "len(grammar_5_forSlide(\"Loops\")) == 2", hive.Show(_l7), hive.Show(_r8))
		_l9 := len(grammar_5_forSlide("Variables"))
		_r10 := 4
		hive.TestAssertCmp(__t, (_l9 == _r10), "len(grammar_5_forSlide(\"Variables\")) == 4", hive.Show(_l9), hive.Show(_r10))
	})
	__t.Run("the chapters add up to the tour", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var total int = 0
		_ = total
		for _, chapter := range slides_8_chapters() {
			_ = chapter
			total = hive.AddInt(total, chapter.Length)
		}
		_l11 := total
		_r12 := len(slides_8_all())
		hive.TestAssertCmp(__t, (_l11 == _r12), "total == len(slides_8_all())", hive.Show(_l11), hive.Show(_r12))
	})
}
