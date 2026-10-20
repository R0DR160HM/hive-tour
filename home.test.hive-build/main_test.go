package main

import (
	"testing"

	"hiveapp/hive"
)

func TestHive(__t *testing.T) {
	hive.InitAtoms([]string{"Nil"})
	__t.Run("every slide the front page points at is in the tour", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var all []slides_8_Slide = slides_8_all()
		_ = all
		var titles []string = []string{}
		_ = titles
		for _, point := range home_9_points() {
			_ = point
			titles = hive.Concat(titles, point.Reads)
		}
		titles = hive.Concat(titles, home_9_proof().Reads)
		for _, title := range titles {
			_ = title
			var found hive.Result[int, string] = main_11_slideIndex(all, title)
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
	__t.Run("the front page fills every hole it leaves, in both languages", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var english string = main_11_homeResponse("en").Body
		_ = english
		var portuguese string = main_11_homeResponse("pt").Body
		_ = portuguese
		hive.TestAssert(__t, (hive.IndexOfStr(english, "HIVE_TOUR_")).IsError(), "indexOf(english, \"HIVE_TOUR_\") is Result.Error(_)")
		hive.TestAssert(__t, (hive.IndexOfStr(portuguese, "HIVE_TOUR_")).IsError(), "indexOf(portuguese, \"HIVE_TOUR_\") is Result.Error(_)")
	})
	__t.Run("a title the tour does not have is an error, not a slide", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		hive.TestAssert(__t, (main_11_slideIndex(slides_8_all(), "No such slide")).IsError(), "main_11_slideIndex(slides_8_all(), \"No such slide\") is Result.Error(_)")
	})
}
