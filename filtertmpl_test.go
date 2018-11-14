package main

import "testing"

func TestFilterTmpl(t *testing.T) {
	input := `title: {{_title_}}
==========
title: {{_title_}}
date: {{_date_}}
tags: [{{_tags_}}]
categories: [{{_categories_}}]
----------`
	expect := `title: {{.Title}}
==========
title: {{.Title}}
date: {{.Date}}
tags: [{{.Tags}}]
categories: [{{.Categories}}]
----------`

	out := filterTmpl(input)
	if expect != out {
		t.Errorf(
			"failed to handle memolist.vim format.\nout:\n%s\n\nexpect:\n%s\n",
			out, expect)
	}
}
