package main

import (
	"testing"

	"hiveapp/hive"
)

func TestHive(__t *testing.T) {
	hive.InitAtoms([]string{"Nil"})
	__t.Run("a paragraph with a code span", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l1 := main_11_proseHtml("a `b` c")
		_r2 := "<div class=\"tour-text\"><p>a <code>b</code> c</p></div>"
		hive.TestAssertCmp(__t, (_l1 == _r2), "main_11_proseHtml(\"a `b` c\") == \"<div class=\\\"tour-text\\\"><p>a <code>b</code> c</p></div>\"", hive.Show(_l1), hive.Show(_r2))
	})
	__t.Run("double backticks hold a single one, padding dropped", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l3 := main_11_inlineHtml("``in `main`:``")
		_r4 := "<code>in `main`:</code>"
		hive.TestAssertCmp(__t, (_l3 == _r4), "main_11_inlineHtml(\"``in `main`:``\") == \"<code>in `main`:</code>\"", hive.Show(_l3), hive.Show(_r4))
		_l5 := main_11_inlineHtml("`` ` ``")
		_r6 := "<code>`</code>"
		hive.TestAssertCmp(__t, (_l5 == _r6), "main_11_inlineHtml(\"`` ` ``\") == \"<code>`</code>\"", hive.Show(_l5), hive.Show(_r6))
	})
	__t.Run("bold, and escaping outside and inside spans", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l7 := main_11_inlineHtml("**x** < `a<b>` & y")
		_r8 := "<strong>x</strong> &lt; <code>a&lt;b&gt;</code> &amp; y"
		hive.TestAssertCmp(__t, (_l7 == _r8), "main_11_inlineHtml(\"**x** < `a<b>` & y\") == \"<strong>x</strong> &lt; <code>a&lt;b&gt;</code> &amp; y\"", hive.Show(_l7), hive.Show(_r8))
	})
	__t.Run("an unclosed backtick is a backtick", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l9 := main_11_inlineHtml("a ` b")
		_r10 := "a ` b"
		hive.TestAssertCmp(__t, (_l9 == _r10), "main_11_inlineHtml(\"a ` b\") == \"a ` b\"", hive.Show(_l9), hive.Show(_r10))
		_l11 := main_11_inlineHtml("``a`")
		_r12 := "``a`"
		hive.TestAssertCmp(__t, (_l11 == _r12), "main_11_inlineHtml(\"``a`\") == \"``a`\"", hive.Show(_l11), hive.Show(_r12))
	})
	__t.Run("stars inside a code span are not bold", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l13 := main_11_inlineHtml("`**`")
		_r14 := "<code>**</code>"
		hive.TestAssertCmp(__t, (_l13 == _r14), "main_11_inlineHtml(\"`**`\") == \"<code>**</code>\"", hive.Show(_l13), hive.Show(_r14))
		_l15 := main_11_inlineHtml("`2 ** 3` is **eight**")
		_r16 := "<code>2 ** 3</code> is <strong>eight</strong>"
		hive.TestAssertCmp(__t, (_l15 == _r16), "main_11_inlineHtml(\"`2 ** 3` is **eight**\") == \"<code>2 ** 3</code> is <strong>eight</strong>\"", hive.Show(_l15), hive.Show(_r16))
	})
	__t.Run("a list, with the lead-in line of its own paragraph", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var got string = main_11_proseHtml("Two:\n\n- `a`, first;\n- b.\n\nAfter.")
		_ = got
		_l17 := got
		_r18 := "<div class=\"tour-text\"><p>Two:</p><ul><li><code>a</code>, first;</li><li>b.</li></ul><p>After.</p></div>"
		hive.TestAssertCmp(__t, (_l17 == _r18), "got == \"<div class=\\\"tour-text\\\"><p>Two:</p><ul><li><code>a</code>, first;</li><li>b.</li></ul><p>After.</p></div>\"", hive.Show(_l17), hive.Show(_r18))
	})
	__t.Run("a lead-in on the line right above a list", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		var got string = main_11_proseHtml("Two:\n- a\n- b")
		_ = got
		_l19 := got
		_r20 := "<div class=\"tour-text\"><p>Two:</p><ul><li>a</li><li>b</li></ul></div>"
		hive.TestAssertCmp(__t, (_l19 == _r20), "got == \"<div class=\\\"tour-text\\\"><p>Two:</p><ul><li>a</li><li>b</li></ul></div>\"", hive.Show(_l19), hive.Show(_r20))
	})
	__t.Run("the description has no markup", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l21 := main_11_plainText("`hive build` **writes** a program.")
		_r22 := "hive build writes a program."
		hive.TestAssertCmp(__t, (_l21 == _r22), "main_11_plainText(\"`hive build` **writes** a program.\") == \"hive build writes a program.\"", hive.Show(_l21), hive.Show(_r22))
	})
	__t.Run("unicode passes through", func(__t *testing.T) {
		defer hive.TestRecover(__t)
		_l23 := main_11_inlineHtml("é — `ç`…")
		_r24 := "é — <code>ç</code>…"
		hive.TestAssertCmp(__t, (_l23 == _r24), "main_11_inlineHtml(\"é — `ç`…\") == \"é — <code>ç</code>…\"", hive.Show(_l23), hive.Show(_r24))
	})
}
