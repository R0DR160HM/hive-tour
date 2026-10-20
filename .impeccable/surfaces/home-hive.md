---
version: 1
slug: "home-hive"
primary_target: "home.hive"
related_targets: ["theme.hive","main.hive","chart.hive"]
---

# Surface: the front page (`/`, `/?lang=pt`)

Mode: **Persuade**. Built code-led (no image generation on this machine).

Audience: evaluators, developers deciding whether Hive suits their next project. Action: start the tour (primary); contents, repository and download are secondary. Proof on hand: real runnable samples, the platform/library/command tables, and the fact that the site is a Hive program. Constraints: no JavaScript; everything embedded in the binary (fonts included); `hive.ui` views plus spliced HTML only where `hive.ui` has no widget; EN/PT-BR parity. The home page may be reordered, merged and trimmed with every fact kept.

What the user said is wrong today: it looks generic. To avoid: a literal bee/honey theme, a SaaS landing page, and losing the calm.

History: the first build set the whole site in railroad-diagram notation (the "Syntax Diagrams" card). The user rejected that ("I thought the tour would have the diagrams, not that the whole site would be diagram-based") and steered on 2026-10-20: the front page is a language homepage done well, the contents is The Wall Chart, the slides keep the diagrams and gain chapter colour, and the colour is chapter hues plus amber.

## Direction contract

THESIS: A language's home page done properly, which ends by showing the whole language at once. The first screen says what Hive is, offers one way on and shows a real program. The foot sets all 74 tour slides out as one periodic-table chart, so "small enough to learn in a day" is something you can see. It refuses the SaaS landing page (gradient hero, feature-card rows, glow) and any railroad drawing outside the tour.

OWN-WORLD: Cool white paper (#f6f7f8, never cream) and one ink. Source Serif 4 for headings and prose, Source Code Pro for programs, both embedded. Programs sit in a quiet panel a shade off the paper, with bold keywords, serif-italic comments and two quiet inks for strings and literals. Nine muted chapter colours that only ever fill a tile. Amber appears only on the one button and where the reader is. Sections are separated by paper; the only hairlines are table rows.

STORY: In one screen the visitor reads what Hive is, sees a whole program, and has one obvious button. Scrolling, they get the reasons as a list whose headings jump to the section arguing each one (never a grid of equal cells), the honest fit and caveats, each reason argued with its program or table, and then the whole tour on one chart. They start the tour.

FIRST VIEWPORT: The running head (mark, The Hive Tour, Contents, EN/PT). Left, five parts of the width: the mark beside "Hive" at about 104px, the lede at about 27px, the intro paragraph, then the ways in on two deliberate lines: the amber "Start the tour" pill with Contents beside it, and under them, quieter, View the repository and Download the compiler. Right, seven parts: the web-service sample in its panel, starting level with the name.

FORM: User-steered fusion of the dealt hand: the canon language-homepage card (this surface), The Wall Chart (the closing chart, from the roll's assigned card) and Syntax Diagrams (kept to the tour only). seed:98480a1a92e51951.

Each reasoned section names its slides as small chapter-coloured tiles, the chart a few tiles at a time. Signature: the chart at the foot, where every tile is a slide link coloured by chapter and read slides turn a deeper shade (`:visited`, no JS). Motion grammar: no entrance animation; a 160ms colour change on the button and the tiles, and nothing moves position.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance
