# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

**Primary: evaluators.** Developers deciding whether Hive is worth their next project. They arrive at the front page with questions: what the language is, what they gain and give up, whether it suits this particular project, and whether any of it is real. They are judging a language that a team will have to live with, so they discount pages that list only strengths.

**Secondary: learners.** The same developers once they have decided, working through the tour one slide at a time, in order, copying examples into a `.hive` file and running them. Some arrive mid-tour from a search or a shared link to a single slide.

Both audiences read in English or Brazilian Portuguese, and the two are equal.

## Product Purpose

The Hive Tour is the public home of the Hive programming language at https://hive-lang.run. The compiler's README sends people here as "the whole language. All of it."

It does two jobs on two surfaces:

- **Front page (`/`)**: makes the case for Hive to someone choosing a language. In order: what Hive is, why someone would pick it, whether it suits this project (caveats included), each reason at length with a program that shows it, what comes in the box, and proof that it is real.
- **Tour (`/1` … `/75`, plus `/contents`)**: teaches the whole language, one feature per slide, in an order where nothing is used before the slide that teaches it.

Success means:

- **Tour completion**: readers go deep into the slides rather than stopping at slide 1.
- **Repository interest**: stars, issues and contributors on `R0DR160HM/hive-lang`.
- **Credibility**: Hive reads as a serious, trustworthy language rather than a hobby project.

Compiler downloads were not named as a success measure, although the download link stays.

## Positioning

Hive is a compiled, garbage-collected language with **no runtime exceptions**. It has no null, nothing is thrown, every vector index is proved in bounds at compile time, and anything that can fail returns a `Result` the compiler makes you check. It is built for distributed systems: a service handles one message at a time, so it needs no locks, and calling a service on another machine is the same line of code as calling one locally, over encrypted and authenticated links. The same code runs on Windows, Linux, macOS, the browser (WASM), a container and Android, and choosing a platform is a build flag. It is small enough to learn in a day.

**The site proves its own claim.** It is a Hive program: an HTTP server written in Hive, laid out with Hive's own `hive.ui`, with no web framework and no JavaScript but the one script that runs a tour example in the reader's browser, shipped as one executable in a container. A neighbouring language's site cannot truthfully say the same thing about itself.

## Operating Context

- Visitors reach the front page from the compiler README, search results, and pasted links that unfurl through Open Graph and Twitter cards. Visitors land on individual slides from search and shared links.
- The tour is read linearly. Navigation is a running head (tour title, Contents, EN/PT switch), a progress bar, and prev/next at the foot of each slide. `/contents` is the map. The site deliberately has no sidebar of all slides.
- Readers edit and run examples on the slide itself (the playground), or copy them and run them with `hive run`.
- Deployment: `hive container main.hive` wrote the Dockerfile, since edited by hand: the runtime image is Debian slim carrying the binary, Go and the compiler (as `hive`), with Go's build cache warmed for `js/wasm`, because the playground compiles there. It runs on Fly.io (`fly.toml`, app `hive-tour`), and the server listens on `:8080`.
- Local development: `hive run main.hive` (compiler v0.2.x plus Go; the playground needs `hive` and `go` on the `PATH`). Tests: `hive test .`.

## Capabilities and Constraints

**Binding for any redesign (confirmed 2026-10-20):**

- **No JavaScript shipped, with one exception.** Interactivity is limited to what HTML and CSS can do, plus server round-trips, everywhere but the playground (confirmed 2026-10-21). The playground is the one script: on a slide with a runnable example, `/play/play.js` makes the example editable, posts it to `/play/compile`, and runs the `js/wasm` program that comes back in a Web Worker, with a 10-second limit and a Stop button. **The server compiles reader code and never runs it, sandboxed or otherwise.** Every page reads in full without the script: the controls are rendered `hidden` and only the script uncovers them, and a slide with nothing runnable, like the front page, carries no script at all. No other feature may use JavaScript. Which programs can run in a browser (no files, databases, connections, terminals, subprocesses, windows or locked `Secret`s; no imports outside `hive.*`) is decided in `play.hive`, and the server refuses the same list.
- **Colour scheme follows the system by default.** A link in the running head lets a reader switch between light and dark. It works without JavaScript: `/theme/dark|light/<page>` sets a one-word `theme` cookie (1 year, HttpOnly, SameSite=Lax, no identifier) and redirects back. Pages then carry `data-theme` on `<html>`. HTML responses send `Vary: Cookie`, and `/theme/` is disallowed in robots.txt.
- **One executable, no externals.** Every byte is served from the binary, except two the playground needs: the programs it compiles, and Go's `wasm_exec.js`, which is read from the Go in the image so it always matches the toolchain that compiled the program. There are no CDNs and no externally hosted fonts, scripts or images. New assets are embedded the way `./icon`, `./logo` and `./shots` embed theirs (SVG paths, or base64 rasters decoded at request time).
- **`hive.ui` views stay the base.** Layout is built from `hive.ui` views and rendered by `ui.page`. Hand-written HTML is spliced in only where `hive.ui` has no widget: the code samples, prose with inline code and bold, and the document head. The design lives in `theme.hive` as a stylesheet layered over `ui.page`'s own stylesheet and targets the `h-*` classes that `hive.ui` emits. `hive.ui` renders view attributes as inline `style`, so responsive overrides of row direction, gap and alignment need `!important`.
- **EN / PT-BR parity.** Every visible string ships in both languages. Portuguese is a full page (`?lang=pt`, `hreflang` pt-BR), not a fallback. Code is never translated; only its `//` comments are (`codePt`).

**Content rules already in force:**

- Every code example is a complete program that compiles. Where a comment says what a line prints, that is what it prints. Front-page samples are kept to about 64 columns.
- Front-page points link to slides by English title, and `home.test.hive` fails the build on a title the tour does not have.
- Slide count is part of the type (`Slide[75]`). The pager, the contents page and the sitemap are all built from `slides.all()`.

**Modules:** `main.hive` (server, routing, views, head, SEO), `home.hive` (front-page copy and samples), `slides.hive` (the tour, about 80% of the program), `theme.hive` (the stylesheet and the syntax-highlighting class names), `highlight.hive` (a Hive lexer that paints samples server-side), `logo.hive` / `icon.hive` (the mark as SVG and as PNG/ICO), `shots.hive` (an embedded screenshot for the 3D-scene slide, which a page cannot render), `play.hive` (the playground: which examples a browser can run, and its one script), `compile.go` (the playground's build: one at a time, with a deadline that kills it, in a scrubbed environment).

**SEO surface to preserve:** canonical and `hreflang` alternates, Open Graph and Twitter cards, JSON-LD (`WebSite` on the front page, `TechArticle` on each slide), `rel=prev/next`, `/robots.txt`, `/sitemap.xml`, and a 404 page marked noindex.

## Brand Commitments

- **Name:** Hive. The site is "The Hive Tour" / "O Tour do Hive".
- **Mark:** an amber bee on a honeycomb (`assets/hive-logo.svg`, also drawn in `logo.hive`). Brand amber `#e29b19` is also the `theme-color`. Alt text: "The Hive logo: a bee on a honeycomb".
- **Voice:** plain, exact, unhyped. Claims are stated as what a project gets ("No crashes in production") and backed by a sentence saying how. Limitations are stated as plainly as strengths. The "Think twice if" section exists because a page that lists only strengths gets discounted.
- **Author:** R0DR160HM. Repository: https://github.com/R0DR160HM/hive-lang. Downloads go through GitHub's `/releases/latest` redirect.

## Evidence on Hand

- The site itself, as a working Hive program (the front page's "The proof" section).
- 77 slides of bilingual explanation and runnable examples (`slides.hive`).
- Front-page samples in `home.hive`: a web service with shared state, a `Result` lookup, and the same service called locally and across machines.
- Factual tables: six platform targets with their build commands, 12 standard-library modules, 5 `hive` commands.
- One real screenshot: a `hive.ui.scene` window running a slide's own code (`/shot/three-dimensions.png`, embedded in `shots.hive`).
- The compiler repository: README, spec, CHANGELOG, and an `examples/` folder whose programs (from `echo` to a multiplayer shooter) are all compiled and run by CI.
- Release state: pre-1.0. The Dockerfile pins v0.2.5, and the local compiler is v0.2.7.

**Absent, and not to be fabricated:** testimonials, named users or companies, adoption figures, star counts, benchmarks, performance comparisons, press, and any stability or support guarantee.

## Product Principles

1. **The site is the proof.** Every technical choice either keeps the "Hive program, no framework, no JavaScript" claim true or is not made. A feature that would need a script tag is the wrong feature; the playground, which cannot run a program in a browser without one, is the single exception, and the claim says so.
2. **Answer the evaluator honestly.** Give gains and costs side by side, and say plainly that Hive is pre-1.0. Credibility comes from candour, not volume.
3. **A tour has an order.** It has a first slide, a last slide, and a sequence that is the argument. Navigation serves moving forward. The map is a place you go to, not furniture you carry around.
4. **Code is the content.** Samples are complete, runnable and readable without horizontal scrolling at the size they ship. Everything else on the page exists to set them up.
5. **Two languages, one site.** Portuguese is never a second-class page.

## Accessibility & Inclusion

- Text contrast is held to at least 4.5:1 in both light and dark themes. `theme.hive` documents this and picks the darker `--accent` over brand amber for text for that reason.
- Every page has the right `<html lang>` (en / pt-BR) so screen readers pick the correct voice.
- The heading structure is meaningful (the reasons are `h3` under `h2` so they can be listed). Focus rings are visible and keyboard-only. The mark and the screenshot have alt text in both languages.
- Light and dark color schemes both follow `prefers-color-scheme`.
