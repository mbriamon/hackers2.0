# bet me if you can
> **Campus trash talk, now with receipts.**  
> An Elm + Go web app that lets students bet on intramural games. Odds are pool-based (home/away/draw) and update in real time as stakes change.

**Demo:** `https://hackers2-0-m6xe6u2td-mbriamons-projects.vercel.app/`  
**Stack:** Elm (SPA) • Go (serverless API) • Vercel (hosting & functions)

---

## Goal of the project
Intramural rivalries are fun; keeping score of “Venmo me, bro” bets isn’t.  
**bet me if you can** provides a tiny, transparent ledger for token stakes on campus games:

- create/list games
- place a stake on home/away/draw
- see **live, pool-based odds**
- settle results and split the pot proportionally

> **MVP note:** Tokens represent dollars (**1 token ≈ \$1**) but this prototype **does not process payments**. Settle off-app (e.g., Venmo/cash). The goal is to demonstrate the interaction model, odds math, and an Elm+Go cloud deployment.

---

## Why Elm + Go (unique language features we leveraged)
Originally chose based on vibes, but here are some cool things:

### Elm (frontend)
One source of truth: Elm keeps all screen data in one place (a single Model).
When you click a button, Elm sends a Message to an update function that returns a new Model. No hidden state = fewer WTFs.

Won’t accept bad API data: We tell Elm exactly what the JSON should look like.
If the backend changes shape, Elm refuses to compile until we fix the decoder. That’s why the UI doesn’t randomly explode.

No “typo strings”: Instead of "home"/"HOME"/"hme", Elm lets us define
Selection = Home | Away | Draw. You literally can’t pass a wrong value.

Doesn’t crash at runtime: Elm is designed so most mistakes are caught while building. If it compiles, it’s usually solid.

So what for our app?
The games list, odds, and bets screens stay in sync, and bad JSON or typos can’t sneak in and break the UI.

### Go (backend)
Tiny functions per URL: Each API route is just a small Go function using the standard library (net/http). Easy to read and test.

No always-on server to babysit; it scales to zero.

No race conditions: Multiple requests can hit at once, so we put a lock (sync.Mutex) around the in-memory data. That stops two bets from editing the same game at the exact same time.

Odds are precise and sent by the API: Odds = each pool / total pool.
We recalculate on every response and send the numbers in JSON. The frontend just shows them—no guessing.

So what for our app?
Placing a bet is fast, safe from data races, and the odds you see right after betting are the authoritative ones from the server. 

---

## Architecture (MVP)

[Browser/Elm SPA]
      │  (fetch /api/*)
      ▼
[Vercel CDN] —— serves /public (index.html + elm.js)
      │
      ▼
[Go Serverless Functions]  — api/router.go → Handler(...)
      │
      ▼
[In-memory store]          — games, bets, wallets (demo)

## Functional Programming (Elm --> Frontend)
Where: frontend/src/Main.elm (your Model / Msg / update / view).

Why it counts: Elm uses the TEA pattern (The Elm Architecture): one immutable state (“Model”), pure update functions, and a view that’s a pure function of state. No hidden state. Basically: Every click sends a message to one function that returns a brand-new state. No random side effects.

Bonus: Elm’s typed JSON decoders refuse bad data from the API at compile time. If we change the API fields, Elm makes us fix the decoder before it will build.

One-liner for slides:
We leveraged Elm’s pure functional architecture (Model/Msg/update) and typed JSON decoders so the UI can’t crash from bad data.

## Functional Programming (Go --> Backend)
Where: api/router.go (type store struct { mu sync.Mutex … }).

Why it counts: On Vercel, multiple HTTP requests hit your function at the same time. We used Go’s concurrency primitive sync.Mutex to lock the shared in-memory store (games/bets/wallets) so two bets don’t stomp each other. In short, our API can handle simultaneous bets safely; a lock stops race conditions.

Also relevant: Go’s serverless model naturally runs requests in parallel; our code is written to be thread-safe.

One-liner for slides: We used Go’s concurrency primitives (sync.Mutex) to make the in-memory pools race-free under concurrent requests.