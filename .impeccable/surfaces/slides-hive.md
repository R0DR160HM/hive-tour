---
version: 1
slug: "slides-hive"
primary_target: "slides.hive"
related_targets: ["theme.hive","main.hive","rail.hive","grammar.hive","chart.hive"]
---

# Surface: the tour (`/1` … `/74`, `/contents`, the 404)

Mode: **Read**. Built code-led.

Audience: learners working through the slides in order, and visitors landing on one slide from search. Task: understand one feature, run its examples, and move on. Constraints: slide prose and code stay word for word, and only the presentation changes. No JavaScript, everything embedded, `hive.ui` views as the base, EN/PT-BR parity.

What the user said is wrong today: reading the tour is tiring, because 74 slides all look the same and progress is hard to feel. The book's calm has to stay.

History: steered by the user on 2026-10-20 after the all-railroad first build. Confirmed for slides: grammar diagrams, chapter colour, track at the top, text beside code. Contents: The Wall Chart.

## Direction contract

THESIS: Each slide is one element of the language. Its tile, in its chapter's colour, stands beside the title; its grammar rules are drawn as railroad diagrams, folded under the title behind one closed line that names them; its explanation sits beside its programs. The contents is the whole language as one chart. It refuses 74 identical pages and the single column that stacks code above prose.

OWN-WORLD: The front page's world. Cool paper and one ink, Source Serif 4 prose at 18px and a 66ch measure, Source Code Pro programs in quiet panels. Railroad diagrams (capsules for signs, square boxes with italic names for rules, 2px track) are drawn by `hive.ui`'s canvas. Nine muted chapter colours fill tiles and nothing else. Amber marks only the current station; a slide's download and repository links are ink-outline pills.

STORY: The reader always knows where they are (the track: 74 stations in nine chapter segments, the current one amber; under it "Control flow and functions" and "Slide 12 of 74"). They see which part of the language this is by the tile's colour, read the explanation beside the programs it explains, and move on along the line.

FIRST VIEWPORT: Running head. The track across the full width, with chapter and slide number under it. Then a row: the slide's tile (about 136px square in the chapter colour, showing number, symbol and chapter name) and the title in display serif. Then, on a slide that teaches grammar, one closed "Grammar: …" line naming its rules (native <details>, no script); opened, the railroad diagrams with their EBNF and spec links. The slide opens on prose and programs, not diagrams (user, 2026-10-20: "The tour is a bit too diagram-heavy" → chose "All folded"). Then two columns that start level: the prose (flexible, at least 340px) and the examples (up to 700px, sized for about 80-column programs at 13.5px). A slide whose widest program line exceeds 84 columns stacks prose above full-width programs. At the foot: previous and next as pills with drawn chevrons, either side of a line carrying the amber station.

FORM: User-steered fusion: Syntax Diagrams (the drawn rules and the track), The Wall Chart (tiles, chapter colours, the contents chart). seed:98480a1a92e51951. Contents: the chart as a skyline of chapter blocks, with the long standard-library chapter set apart below as detached rows and an empty dashed-outline tile in its chapter's colour where it belongs. Read slides are filled in the deeper chapter shade via `:visited`; below 1180px each chapter is a block under its own name.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
