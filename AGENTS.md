# aura-forge — Project Mission & Engineering Guardrails

## 1. Core Mission (North Star)
- **Primary Goal:** Help the project owner (as a beginner EVE Online player) and their piloting workflow start, manage, and scale an **industry (manufacturing)** and **marketplace trading** business in EVE Online toward **hundreds of billions of ISK (100B+ ISK)** in assets.
- **Pragmatic Technology Openness:** Any technology, library, algorithm, or feature may be proposed and adopted as long as it tangibly helps achieve asset growth, margin safety, and operational simplicity for a beginner.
- **Beginner-First Decision Support:** Every new feature must guide concrete decisions (protecting against capital lock-up, slow movers, unprofitable undercuts, and hidden tax/relist fees) with clear, beginner-friendly guidance.

## 2. Language Convention
- **English for Code & Docs; Bilingual EN/ID for UI:** All code, comments, documentation (`README.md`, `docs/`, `AGENTS.md`), SQL migrations, and commit messages MUST be written in **English**. User-facing UI copy supports **bilingual EN (default) & ID** via `internal/i18n` (`af_lang` cookie), while keeping **sidebar navigation menus** and **EVE Online in-game terms** (`Sell Order`, `Buy Order`, `Blueprint`, `Runs`, `Broker Fee`, `Sales Tax`, `Margin`, `ISK`, `ME`, `TE`, `BPO`, `BPC`, `SCI`, `ROI`, `PI`, `LP`, `ESI`, `SDE`, `T1/T2/T3`, `m³`) untranslated in English. Avoid short cryptic abbreviations (`u`, `tx`, `Qty`, `Est.`, `Avg`, `Calc`) — spell out full words (`units`, `orders`, `Quantity`, `Estimated`, `Average`, `Calculate`) whenever they are concise.
- **Indonesian for AI-to-User Communication & Plans:** Direct AI-to-user conversation in chat AND all AI-generated review artifacts presented to the user (including implementation plans, learning proposals, and walkthroughs) MUST be written in **Indonesian**.

## 3. Product, Auth & ESI Guardrails
- **App Auth vs. Optional EVE SSO:**
  - The login page (`/login`) is a standard internal login (`Username` & `Password`) with **no public registration** — only the `admin` role can create and manage `user`/`admin` accounts via **User Management**.
  - **EVE Online SSO** is **optional (plug-and-play)**: integration buttons and related menus appear only after login and only when `ESI_CLIENT_ID` and `ESI_CLIENT_SECRET` are configured in `.env`.
- **Public ESI Anti-Blocking:** All EVE ESI calls must include a custom `User-Agent`, PostgreSQL caching (`esi_cache` with `Expires`/`ETag`/`304`), in-flight deduplication (`singleflight`), `X-Pages` pagination limits, and strict adherence to `X-ESI-Error-Limit-Remain` and `Retry-After`.

## 4. UI & Visual Consistency
- Preserve the **Seamless Deep Orbital Space** theme (`#0d1021 → #171530 → #0b0d1a`) with a borderless transparent sidebar, continuous planet horizon backdrop flowing from sidebar into `main`, *Royal Industrial Aura Violet Banner*, adaptive glass cards (*Solusi B*), and FOUC guard + `x-cloak` for blink-free refreshes.

## 5. Architecture & Verification Standards
- Maintain **Clean Architecture** (Dependency Inversion via interfaces in `internal/domain`), `NUMERIC(20,2)` precision for ISK values in PostgreSQL, and all SQL migration files in the root `migrations/` directory.
- **Go Concurrency First (Safe Goroutines):** Proactively maximize goroutines (`sync.WaitGroup`, bounded worker pools, and supervised background workers) for independent I/O, multi-hub ESI fetches, parallel engine fan-outs, and batch evaluations whenever it meaningfully reduces latency. Always enforce bounded concurrency (semaphore caps for ESI/HTTP), `context.Context` cancellation/timeouts, race-free synchronization (`sync.Mutex`/`sync.RWMutex`), and Public ESI anti-blocking limits to prevent goroutine leaks or rate-limit bans.
- **Mandatory Docker Compose Verification:** After code changes, always run `templ generate`, unit tests (`go test`), `go build ./...`, and rebuild & verify live containers via `docker compose up -d --build --force-recreate`.
- **Wiki Synchronization:** Whenever a task adds, changes, or removes user-facing features, workflows, metrics, or settings, check whether the in-app Wiki (`internal/web/wiki.templ`) needs updating and update it to stay aligned with the live behavior.
- **Post-Task Commit Message:** After finishing any code, config, or documentation change and completing verification, always provide a concise, ready-to-use **English** Conventional Commit message (e.g., `feat(scope): ...`, `fix(scope): ...`, `docs(scope): ...`) in the final response.
