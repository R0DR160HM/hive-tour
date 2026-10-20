package main

import (
	"testing"

	"hiveapp/hive"
)

func TestHive(__t *testing.T) {
	hive.InitAtoms([]string{"Nil"})
	__t.Run("every slide has a symbol", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		for _, slide := range slides_1_all() {
			_ = slide
			if (chart_0_symbol(slide.Title) == "") {
				hive.Echo("no symbol for \"" + slide.Title + "\"")
			}
			_l1 := chart_0_symbol(slide.Title)
			_r2 := ""
			hive.TestAssertCmp(__t, !((_l1 == _r2)), "chart_0_symbol(slide.title) != \"\"", hive.Show(_l1), hive.Show(_r2))
		}
	})
	__t.Run("every symbol belongs to a slide", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var titles []string = []string{}
		_ = titles
		for _, slide := range slides_1_all() {
			_ = slide
			titles = append(titles, slide.Title)
		}
		for _, entry := range chart_0_symbols() {
			_ = entry
			var found hive.Result[int, bool] = hive.IndexOfVec(titles, entry.Title)
			_ = found
			{
				_m3 := false
				if _t4 := found; _t4.IsError() {
					_m3 = true
					hive.Echo("a symbol for a slide the tour does not have: \"" + entry.Title + "\"")
				}
				_ = _m3
			}
			hive.TestAssert(__t, (found).IsOk(), "found is Result.Ok(_)")
		}
	})
}
