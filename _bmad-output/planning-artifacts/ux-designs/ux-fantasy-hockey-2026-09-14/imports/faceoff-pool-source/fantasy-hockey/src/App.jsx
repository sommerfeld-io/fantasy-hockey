import React, { useState, useMemo } from "react";
import {
  Trophy, Target, Users, ChevronRight, Clock, Check, Lock,
  Medal, ArrowLeft, ListChecks, CircleDot,
} from "lucide-react";

/* ------------------------------------------------------------------ *
 * NHL Prediction Game — mobile-first dark prototype
 * Private game: Basti (you), Sadl, Tobbi
 * All data is in-memory sample data.
 * ------------------------------------------------------------------ */

// Arena-night palette (inline hex so it isn't the stock slate/sky look)
const c = {
  bg: "#0d1117",
  surface: "#161b22",
  raised: "#21262d",
  border: "#30363d",
  borderSoft: "#21262d",
  text: "#e6edf3",
  muted: "#8b949e",
  faint: "#6e7681",
  ice: "#58a6ff",           // accent — used sparingly
  iceDeep: "#1f6feb",
  sel: "#30363d",           // neutral "selected" fill
  goal: "#f85149",
  gold: "#d29922",
  green: "#3fb950",
  greenBtn: "#238636",
  greenBtnBorder: "#2ea043",
};

/* ---------------------------- team data ---------------------------- */
const T = (id, name, abbr) => ({ id, name, abbr });
const DIVISIONS = {
  atlantic: {
    label: "Atlantic", conf: "east",
    teams: [
      T("BOS", "Boston Bruins", "BOS"), T("BUF", "Buffalo Sabres", "BUF"),
      T("DET", "Detroit Red Wings", "DET"), T("FLA", "Florida Panthers", "FLA"),
      T("MTL", "Montréal Canadiens", "MTL"), T("OTT", "Ottawa Senators", "OTT"),
      T("TBL", "Tampa Bay Lightning", "TBL"), T("TOR", "Toronto Maple Leafs", "TOR"),
    ],
  },
  metro: {
    label: "Metropolitan", conf: "east",
    teams: [
      T("CAR", "Carolina Hurricanes", "CAR"), T("CBJ", "Columbus Blue Jackets", "CBJ"),
      T("NJD", "New Jersey Devils", "NJD"), T("NYI", "New York Islanders", "NYI"),
      T("NYR", "New York Rangers", "NYR"), T("PHI", "Philadelphia Flyers", "PHI"),
      T("PIT", "Pittsburgh Penguins", "PIT"), T("WSH", "Washington Capitals", "WSH"),
    ],
  },
  central: {
    label: "Central", conf: "west",
    teams: [
      T("CHI", "Chicago Blackhawks", "CHI"), T("COL", "Colorado Avalanche", "COL"),
      T("DAL", "Dallas Stars", "DAL"), T("MIN", "Minnesota Wild", "MIN"),
      T("NSH", "Nashville Predators", "NSH"), T("STL", "St. Louis Blues", "STL"),
      T("UTA", "Utah Mammoth", "UTA"), T("WPG", "Winnipeg Jets", "WPG"),
    ],
  },
  pacific: {
    label: "Pacific", conf: "west",
    teams: [
      T("ANA", "Anaheim Ducks", "ANA"), T("CGY", "Calgary Flames", "CGY"),
      T("EDM", "Edmonton Oilers", "EDM"), T("LAK", "Los Angeles Kings", "LAK"),
      T("SJS", "San Jose Sharks", "SJS"), T("SEA", "Seattle Kraken", "SEA"),
      T("VAN", "Vancouver Canucks", "VAN"), T("VGK", "Vegas Golden Knights", "VGK"),
    ],
  },
};
const DIV_KEYS = ["atlantic", "metro", "central", "pacific"];
const ALL_TEAMS = DIV_KEYS.flatMap((k) => DIVISIONS[k].teams);
const teamById = Object.fromEntries(ALL_TEAMS.map((t) => [t.id, t]));
const CONF = {
  east: { label: "Eastern", divs: ["atlantic", "metro"] },
  west: { label: "Western", divs: ["central", "pacific"] },
};
const teamConfById = Object.fromEntries(
  DIV_KEYS.flatMap((k) => DIVISIONS[k].teams.map((t) => [t.id, DIVISIONS[k].conf]))
);

/* ------------------------- player suggestions ------------------------- */
const SKATERS = [
  "Connor McDavid", "Leon Draisaitl", "Nathan MacKinnon", "Auston Matthews",
  "Nikita Kucherov", "David Pastrňák", "Mikko Rantanen", "Kirill Kaprizov",
  "Jack Eichel", "Mitch Marner", "Matthew Tkachuk", "Artemi Panarin",
  "Jack Hughes", "Sebastian Aho", "Cale Makar", "Quinn Hughes",
];
const DEFENSEMEN = [
  "Cale Makar", "Quinn Hughes", "Victor Hedman", "Roman Josi", "Adam Fox",
  "Zach Werenski", "Rasmus Dahlin", "Josh Morrissey", "Evan Bouchard", "Miro Heiskanen",
];
const GOALIES = [
  "Connor Hellebuyck", "Igor Shesterkin", "Sergei Bobrovsky", "Ilya Sorokin",
  "Andrei Vasilevskiy", "Jeremy Swayman", "Juuse Saros", "Jake Oettinger",
  "Adin Hill", "Stuart Skinner",
];

/* --------------------------- players --------------------------- */
const PLAYERS = [
  { id: "basti", name: "Basti", you: true },
  { id: "sadl", name: "Sadl" },
  { id: "tobbi", name: "Tobbi" },
];

/* -------------------- prediction set definitions -------------------- */
const SETS = [
  { id: "cup", title: "Cup champion", subtitle: "Your Stanley Cup winner", deadline: "2026-10-06T19:00:00", phase: "season", kind: "cup" },
  { id: "presidents", title: "Presidents' Trophy", subtitle: "Best regular-season record", deadline: "2026-10-06T19:00:00", phase: "season", kind: "presidents" },
  { id: "divisions", title: "Division picks", subtitle: "Playoff teams & division winners", deadline: "2026-10-06T19:00:00", phase: "season", kind: "divisions" },
  { id: "awards", title: "Player awards", subtitle: "Hart, Norris, Vezina, Art Ross, Rocket", deadline: "2026-10-06T19:00:00", phase: "season", kind: "awards" },
  { id: "playoffcup", title: "Playoffs Cup pick", subtitle: "Re-pick the Stanley Cup winner", deadline: "2027-04-15T18:00:00", phase: "playoffs", kind: "cup" },
  { id: "r1", title: "Playoff round 1", subtitle: "8 series — winner & length", deadline: "2027-04-18T18:00:00", phase: "playoffs", kind: "round" },
  { id: "r2", title: "Playoff round 2", subtitle: "Set once round 1 ends", deadline: "2027-04-30T18:00:00", phase: "playoffs", kind: "round", upcoming: true },
  { id: "cf", title: "Conference finals", subtitle: "Set once round 2 ends", deadline: "2027-05-14T18:00:00", phase: "playoffs", kind: "round", upcoming: true },
  { id: "scf", title: "Stanley Cup final", subtitle: "Set once the finalists are known", deadline: "2027-05-28T18:00:00", phase: "playoffs", kind: "round", upcoming: true },
];

// Seeded round-1 matchups (higher seed listed first)
const R1_SERIES = [
  { id: "s1", a: "FLA", b: "OTT" }, { id: "s2", a: "TOR", b: "TBL" },
  { id: "s3", a: "WSH", b: "NYR" }, { id: "s4", a: "CAR", b: "NJD" },
  { id: "s5", a: "WPG", b: "STL" }, { id: "s6", a: "DAL", b: "COL" },
  { id: "s7", a: "VGK", b: "EDM" }, { id: "s8", a: "LAK", b: "VAN" },
];

/* ---------------------- seeded opponent picks ---------------------- */
const S = { submitted: true };
const SEED = {
  sadl: {
    cup: { ...S, cup: "FLA" },
    presidents: { ...S, presidents: "DAL" },
    divisions: { ...S,
      playoffTeams: {
        atlantic: ["FLA", "TOR", "TBL", "OTT"], metro: ["CAR", "WSH", "NJD", "NYR"],
        central: ["DAL", "WPG", "COL", "MIN", "UTA"], pacific: ["VGK", "EDM", "LAK"],
      },
      divWinners: { atlantic: "FLA", metro: "CAR", central: "DAL", pacific: "VGK" },
    },
    awards: { ...S,
      hart: ["Connor McDavid", "Nathan MacKinnon", "Nikita Kucherov"],
      norris: ["Cale Makar", "Quinn Hughes", "Zach Werenski"],
      vezina: ["Connor Hellebuyck", "Igor Shesterkin", "Andrei Vasilevskiy"],
      artRoss: ["Nikita Kucherov", "Connor McDavid", "Nathan MacKinnon"],
      rocket: ["Auston Matthews", "Leon Draisaitl", "Sam Reinhart"],
    },
    playoffcup: { ...S, cup: "DAL" },
    r1: { ...S, series: { s1: { w: "FLA", g: 5 }, s2: { w: "TOR", g: 6 }, s3: { w: "WSH", g: 6 }, s4: { w: "CAR", g: 5 }, s5: { w: "WPG", g: 7 }, s6: { w: "DAL", g: 6 }, s7: { w: "VGK", g: 7 }, s8: { w: "LAK", g: 6 } } },
  },
  tobbi: {
    cup: { ...S, cup: "EDM" },
    presidents: { ...S, presidents: "EDM" },
    divisions: { ...S,
      playoffTeams: {
        atlantic: ["FLA", "TOR", "TBL", "BOS"], metro: ["CAR", "WSH", "NYR", "NJD", "PIT"],
        central: ["DAL", "COL", "WPG"], pacific: ["VGK", "EDM", "LAK", "VAN", "SEA"],
      },
      divWinners: { atlantic: "TOR", metro: "WSH", central: "COL", pacific: "EDM" },
    },
    awards: { ...S,
      hart: ["Leon Draisaitl", "Connor McDavid", "Auston Matthews"],
      norris: ["Quinn Hughes", "Victor Hedman", "Cale Makar"],
      vezina: ["Igor Shesterkin", "Jeremy Swayman", "Connor Hellebuyck"],
      artRoss: ["Connor McDavid", "Leon Draisaitl", "Kirill Kaprizov"],
      rocket: ["Leon Draisaitl", "Auston Matthews", "Kirill Kaprizov"],
    },
    playoffcup: { ...S, cup: "EDM" },
    r1: { ...S, series: { s1: { w: "FLA", g: 6 }, s2: { w: "TOR", g: 5 }, s3: { w: "NYR", g: 7 }, s4: { w: "CAR", g: 4 }, s5: { w: "WPG", g: 6 }, s6: { w: "COL", g: 7 }, s7: { w: "EDM", g: 6 }, s8: { w: "LAK", g: 7 } } },
  },
  basti: {}, // you fill these
};

// Sample standings (prototype scoring)
const STANDINGS = {
  basti: { rs: 42, po: 15 },
  sadl: { rs: 39, po: 22 },
  tobbi: { rs: 51, po: 9 },
};

/* ---------------------------- helpers ---------------------------- */
function fmtDeadline(iso) {
  const d = new Date(iso);
  return d.toLocaleString("en-US", {
    weekday: "short", month: "short", day: "numeric", year: "numeric",
    hour: "numeric", minute: "2-digit",
  }).replace(",", "");
}
function relDays(iso) {
  const ms = new Date(iso) - new Date();
  const days = Math.round(ms / 86400000);
  if (ms < 0) return "closed";
  if (days === 0) return "today";
  if (days === 1) return "tomorrow";
  return `in ${days} days`;
}
// A conference must total exactly 8 across its two divisions, each division 3–5.
// Selection is capped during input, so total === 8 implies a valid 4/4 or 5/3 split.
function confTotal(pt, conf) {
  return CONF[conf].divs.reduce((n, k) => n + (pt[k]?.length || 0), 0);
}
function playoffTeamsValid(pt) {
  return confTotal(pt, "east") === 8 && confTotal(pt, "west") === 8;
}

/* ============================= APP ============================= */
export default function App() {
  const [tab, setTab] = useState("predict");
  const userId = "basti";
  const [openSet, setOpenSet] = useState(null);
  const [preds, setPreds] = useState(() => JSON.parse(JSON.stringify(SEED)));

  const user = PLAYERS.find((p) => p.id === userId);
  const save = (setId, data) =>
    setPreds((p) => ({ ...p, [userId]: { ...p[userId], [setId]: { ...data, submitted: true } } }));

  return (
    <div className="overflow-hidden" style={{ background: c.bg, color: c.text, height: "100dvh", fontFamily: "ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, sans-serif" }}>
      <div className="relative mx-auto flex h-full max-w-md flex-col overflow-hidden" style={{ background: c.bg }}>
        <Header user={user} />

        <main className="flex-1 overflow-y-auto px-4 pb-6 pt-2">
          {tab === "predict" && <PredictTab userId={userId} preds={preds} onOpen={setOpenSet} />}
          {tab === "board" && <BoardTab />}
          {tab === "compare" && <CompareTab preds={preds} />}
        </main>

        <BottomNav tab={tab} setTab={setTab} />

        {openSet && (
          <SetSheet
            set={SETS.find((s) => s.id === openSet)}
            user={user}
            data={preds[userId]?.[openSet]}
            onClose={() => setOpenSet(null)}
            onSave={(d) => { save(openSet, d); setOpenSet(null); }}
          />
        )}
      </div>
    </div>
  );
}

/* ---------------------------- header ---------------------------- */
function Header({ user }) {
  return (
    <header className="shrink-0 z-20 flex items-center justify-between px-4 py-3.5"
      style={{ background: c.surface, borderBottom: `1px solid ${c.border}` }}>
      <div className="text-[16px] font-bold tracking-tight">{user.name}</div>
      <div className="text-[12px] font-semibold" style={{ color: c.muted }}>NHL 2026–27</div>
    </header>
  );
}

/* --------------------------- predict tab --------------------------- */
function PredictTab({ userId, preds, onOpen }) {
  return (
    <div>
      <SectionLabel icon={<Target size={14} />}>Before the season</SectionLabel>
      {SETS.filter((s) => s.phase === "season").map((s) => (
        <SetRow key={s.id} set={s} data={preds[userId]?.[s.id]} onOpen={onOpen} />
      ))}
      <SectionLabel icon={<Trophy size={14} />}>Playoffs</SectionLabel>
      {SETS.filter((s) => s.phase === "playoffs").map((s) => (
        <SetRow key={s.id} set={s} data={preds[userId]?.[s.id]} onOpen={onOpen} />
      ))}
    </div>
  );
}

function SectionLabel({ icon, children }) {
  return (
    <div className="mb-2 mt-4 flex items-center gap-1.5 text-[12px] font-semibold" style={{ color: c.muted }}>
      {icon}{children}
    </div>
  );
}

function StatusPill({ set, data }) {
  const closed = new Date(set.deadline) < new Date();
  let label = "Open", bg = "rgba(88,166,255,0.14)", col = c.ice;
  if (set.upcoming) { label = "Upcoming"; bg = c.raised; col = c.faint; }
  else if (closed) { label = "Closed"; bg = "rgba(248,81,73,0.15)"; col = c.goal; }
  else if (data?.submitted) { label = "Submitted"; bg = "rgba(63,185,80,0.15)"; col = c.green; }
  return (
    <span className="rounded-md px-2 py-0.5 text-[11px] font-semibold" style={{ background: bg, color: col }}>{label}</span>
  );
}

function SetRow({ set, data, onOpen }) {
  const disabled = set.upcoming;
  const accent = set.upcoming ? c.border : data?.submitted ? c.green : c.ice;
  return (
    <button
      disabled={disabled}
      onClick={() => !disabled && onOpen(set.id)}
      className="mb-2 flex w-full items-center gap-3 overflow-hidden rounded-xl text-left"
      style={{ background: c.surface, border: `1px solid ${c.border}`, opacity: disabled ? 0.55 : 1 }}>
      <span style={{ width: 3, alignSelf: "stretch", background: accent }} />
      <div className="min-w-0 flex-1 py-3">
        <div className="flex items-center gap-2">
          <span className="truncate text-[15px] font-bold tracking-tight">{set.title}</span>
          <StatusPill set={set} data={data} />
        </div>
        <div className="truncate text-[12px]" style={{ color: c.faint }}>{set.subtitle}</div>
        <div className="mt-1.5 flex items-center gap-1 text-[11px]" style={{ color: c.muted }}>
          <Clock size={12} />
          <span>{fmtDeadline(set.deadline)}</span>
          <span style={{ color: set.upcoming ? c.faint : c.ice }}>· {relDays(set.deadline)}</span>
        </div>
      </div>
      {disabled ? <Lock size={16} color={c.faint} className="mr-3" /> : <ChevronRight size={18} color={c.muted} className="mr-3" />}
    </button>
  );
}

/* ---------------------------- board tab ---------------------------- */
function BoardTab() {
  const rows = PLAYERS.map((p) => {
    const s = STANDINGS[p.id];
    return { ...p, rs: s.rs, po: s.po, total: s.rs + s.po };
  }).sort((a, b) => b.total - a.total);

  return (
    <div>
      <SectionLabel icon={<Medal size={14} />}>Leaderboard</SectionLabel>
      <p className="mb-2 text-[12px]" style={{ color: c.faint }}>Ranked by total points.</p>
      <div className="overflow-hidden rounded-xl" style={{ background: c.surface, border: `1px solid ${c.border}` }}>
        <div className="items-stretch text-[11px] font-semibold" style={{ display: "grid", gridTemplateColumns: "1.5fr 1fr 1fr 1.1fr", borderBottom: `1px solid ${c.border}` }}>
          <span className="px-3 py-2" style={{ color: c.muted }}>Player</span>
          <span className="px-2 py-2 text-right" style={{ color: c.muted }}>Regular</span>
          <span className="px-2 py-2 text-right" style={{ color: c.muted }}>Playoff</span>
          <span className="px-3 py-2 text-right" style={{ color: c.text, background: "rgba(88,166,255,0.08)", borderLeft: `1px solid ${c.border}` }}>Total</span>
        </div>
        {rows.map((r, i) => {
          const leader = i === 0;
          return (
            <div key={r.id} className="items-center"
              style={{ display: "grid", gridTemplateColumns: "1.5fr 1fr 1fr 1.1fr", borderBottom: i < rows.length - 1 ? `1px solid ${c.borderSoft}` : "none" }}>
              <span className="flex items-center gap-2 px-3 py-3">
                <span className="grid h-6 w-6 place-items-center rounded-full text-[12px] font-bold"
                  style={{ background: leader ? c.gold : c.raised, color: leader ? "#1c1600" : c.muted }}>{i + 1}</span>
                <span className="text-[14px] font-semibold">{r.name}</span>
              </span>
              <span className="px-2 py-3 text-right text-[14px]" style={{ color: c.muted, fontVariantNumeric: "tabular-nums" }}>{r.rs}</span>
              <span className="px-2 py-3 text-right text-[14px]" style={{ color: c.muted, fontVariantNumeric: "tabular-nums" }}>{r.po}</span>
              <span className="px-3 py-3 text-right text-[19px] font-extrabold"
                style={{ color: leader ? c.gold : c.text, background: "rgba(88,166,255,0.08)", borderLeft: `1px solid ${c.border}`, fontVariantNumeric: "tabular-nums", fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{r.total}</span>
            </div>
          );
        })}
      </div>
      <p className="mt-3 text-[11px]" style={{ color: c.faint }}>
        Sample scoring. Regular-season picks feed the Regular column, playoff-round picks feed Playoff, and Total is what decides the standings.
      </p>
    </div>
  );
}

/* --------------------------- compare tab --------------------------- */
function CompareTab({ preds }) {
  const [setId, setSetId] = useState("cup");
  const set = SETS.find((s) => s.id === setId);
  const options = SETS.filter((s) => !s.upcoming);

  return (
    <div>
      <SectionLabel icon={<Users size={14} />}>Everyone's picks</SectionLabel>
      <div className="mb-3 space-y-2">
        {[{ phase: "season", label: "Before the season" }, { phase: "playoffs", label: "Playoffs" }].map((grp) => (
          <div key={grp.phase}>
            <div className="mb-1 text-[11px] font-semibold" style={{ color: c.faint }}>{grp.label}</div>
            <div className="flex flex-wrap gap-2">
              {options.filter((s) => s.phase === grp.phase).map((s) => {
                const on = s.id === setId;
                return (
                  <button key={s.id} onClick={() => setSetId(s.id)}
                    className="rounded-lg px-3 py-1.5 text-[12px] font-semibold"
                    style={{ background: on ? c.sel : c.surface, color: on ? c.text : c.muted, border: `1px solid ${on ? c.ice : c.border}` }}>
                    {s.title}
                  </button>
                );
              })}
            </div>
          </div>
        ))}
      </div>

      <div className="mb-2 flex items-center gap-1 text-[11px]" style={{ color: c.muted }}>
        <Clock size={12} /> Deadline {fmtDeadline(set.deadline)}
      </div>

      <div className="overflow-hidden rounded-xl" style={{ background: c.surface, border: `1px solid ${c.border}` }}>
        {/* table headings — one column per player */}
        <div className="grid grid-cols-3 gap-2 px-3 py-2.5" style={{ background: c.raised, borderBottom: `1px solid ${c.border}` }}>
          {PLAYERS.map((p) => (
            <div key={p.id} className="text-[13px] font-bold" style={{ color: p.you ? c.ice : c.text }}>{p.name}</div>
          ))}
        </div>

        {set.kind === "divisions" && <DivisionsCompare preds={preds} setId={setId} />}
        {set.kind === "awards" && <AwardsCompare preds={preds} setId={setId} />}
        {set.kind === "presidents" && (
          <CompareBlock title="Presidents' Trophy">
            <CompareRow render={(pid) => teamName(preds[pid]?.[setId]?.presidents)} />
          </CompareBlock>
        )}
        {set.kind === "cup" && (
          <CompareBlock title="Stanley Cup winner">
            <CompareRow render={(pid) => teamName(preds[pid]?.[setId]?.cup)} />
          </CompareBlock>
        )}
        {set.kind === "round" && <RoundCompare preds={preds} setId={setId} />}
      </div>
    </div>
  );
}

const teamName = (id) => (id ? teamById[id]?.name ?? id : "—");

// A labelled row inside the compare table (category label + one cell per player).
function CompareBlock({ title, children }) {
  return (
    <div className="px-3 py-2.5" style={{ borderTop: `1px solid ${c.borderSoft}` }}>
      <div className="mb-1.5 text-[11px] font-semibold" style={{ color: c.faint }}>{title}</div>
      {children}
    </div>
  );
}

// Renders a 3-col row; `render(playerId)` returns a string or JSX
function CompareRow({ render }) {
  return (
    <div className="grid grid-cols-3 gap-2">
      {PLAYERS.map((p) => (
        <div key={p.id} className="text-[13px]" style={{ color: p.you ? c.text : c.muted }}>{render(p.id)}</div>
      ))}
    </div>
  );
}

function ListCol({ items }) {
  if (!items || !items.length) return <span style={{ color: c.faint }}>—</span>;
  return (
    <div className="space-y-0.5">
      {items.map((x, i) => <div key={i} className="text-[12px] leading-snug">{x}</div>)}
    </div>
  );
}

// Consistent tag style for team-abbreviation pick values.
function Tag({ children }) {
  return (
    <span className="inline-block rounded px-1 py-0.5 text-[11px] font-semibold"
      style={{ background: c.raised, color: c.muted, fontVariantNumeric: "tabular-nums" }}>
      {children}
    </span>
  );
}

function DivisionsCompare({ preds, setId }) {
  const get = (pid) => preds[pid]?.[setId];
  return (
    <>
      {DIV_KEYS.map((k) => (
        <React.Fragment key={k}>
          <CompareBlock title={`${DIVISIONS[k].label} — playoff teams`}>
            <CompareRow render={(pid) => {
              const ids = get(pid)?.playoffTeams?.[k];
              if (!ids || !ids.length) return <span style={{ color: c.faint }}>—</span>;
              return (
                <div className="flex flex-wrap gap-1">
                  {ids.map((id) => <Tag key={id}>{teamById[id]?.abbr ?? id}</Tag>)}
                </div>
              );
            }} />
          </CompareBlock>
          <CompareBlock title={`${DIVISIONS[k].label} — winner`}>
            <CompareRow render={(pid) => {
              const w = get(pid)?.divWinners?.[k];
              return w ? <Tag>{teamById[w]?.abbr ?? w}</Tag> : <span style={{ color: c.faint }}>—</span>;
            }} />
          </CompareBlock>
        </React.Fragment>
      ))}
    </>
  );
}

function AwardsCompare({ preds, setId }) {
  const get = (pid) => preds[pid]?.[setId];
  const trophy = (key) => <CompareRow render={(pid) => <ListCol items={get(pid)?.[key]} />} />;
  return (
    <>
      <CompareBlock title="Hart finalists">{trophy("hart")}</CompareBlock>
      <CompareBlock title="Norris finalists">{trophy("norris")}</CompareBlock>
      <CompareBlock title="Vezina finalists">{trophy("vezina")}</CompareBlock>
      <CompareBlock title="Art Ross finalists">{trophy("artRoss")}</CompareBlock>
      <CompareBlock title="Rocket Richard finalists">{trophy("rocket")}</CompareBlock>
    </>
  );
}

function RoundCompare({ preds, setId }) {
  if (setId !== "r1") return <EmptyNote>Matchups for this round aren't set yet.</EmptyNote>;
  return (
    <>
      {R1_SERIES.map((s) => (
        <CompareBlock key={s.id} title={`${CONF[teamConfById[s.a]].label} · ${teamById[s.a].abbr} vs ${teamById[s.b].abbr}`}>
          <CompareRow render={(pid) => {
            const pick = preds[pid]?.r1?.series?.[s.id];
            if (!pick) return <span style={{ color: c.faint }}>—</span>;
            return <span className="text-[13px]"><Tag>{teamById[pick.w]?.abbr}</Tag> <span style={{ color: c.muted }}>in {pick.g}</span></span>;
          }} />
        </CompareBlock>
      ))}
    </>
  );
}

function EmptyNote({ children }) {
  return <div className="rounded-xl px-4 py-6 text-center text-[13px]" style={{ background: c.surface, border: `1px dashed ${c.border}`, color: c.faint }}>{children}</div>;
}

/* ============================ THE SHEET ============================ */
function SetSheet({ set, user, data, onClose, onSave }) {
  const closed = new Date(set.deadline) < new Date();
  const [draft, setDraft] = useState(() => data ? JSON.parse(JSON.stringify(data)) : initDraft(set));
  const canSubmit = set.kind !== "divisions" || playoffTeamsValid(draft.playoffTeams);

  return (
    <div className="absolute inset-0 z-40 flex flex-col" style={{ background: c.bg }}>
      <div className="shrink-0 z-10 flex items-center gap-3 px-4 py-3" style={{ background: c.surface, borderBottom: `1px solid ${c.border}` }}>
        <button onClick={onClose} className="grid h-9 w-9 place-items-center rounded-lg" style={{ background: c.raised, border: `1px solid ${c.border}` }}>
          <ArrowLeft size={18} color={c.text} />
        </button>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[15px] font-bold">{set.title}</div>
          <div className="flex items-center gap-1 text-[11px]" style={{ color: c.muted }}>
            <Clock size={11} /> {fmtDeadline(set.deadline)} · {relDays(set.deadline)}
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-4 pb-24 pt-3">
        {closed && <div className="mb-3 rounded-lg px-3 py-2 text-[12px]" style={{ background: "rgba(248,81,73,0.12)", color: c.goal }}>This set is closed — showing a read-only view.</div>}
        {set.kind === "divisions" && <DivisionForm draft={draft} setDraft={setDraft} disabled={closed} />}
        {set.kind === "awards" && <AwardsForm draft={draft} setDraft={setDraft} disabled={closed} />}
        {set.kind === "cup" && <TeamPickForm draft={draft} setDraft={setDraft} disabled={closed} field="cup" title="Stanley Cup winner" hint="Lock in your champion for the whole season." />}
        {set.kind === "presidents" && <TeamPickForm draft={draft} setDraft={setDraft} disabled={closed} field="presidents" title="Presidents' Trophy winner" hint="The team you expect to finish with the league's best record." />}
        {set.kind === "round" && <RoundForm draft={draft} setDraft={setDraft} disabled={closed} set={set} />}
      </div>

      {!closed && (
        <div className="absolute bottom-0 left-0 right-0 px-4 py-3" style={{ background: c.surface, borderTop: `1px solid ${c.border}` }}>
          <button onClick={() => canSubmit && onSave(draft)} disabled={!canSubmit}
            className="w-full rounded-xl py-3 text-[15px] font-bold"
            style={{ background: canSubmit ? c.greenBtn : c.raised, color: canSubmit ? "#ffffff" : c.faint, border: `1px solid ${canSubmit ? c.greenBtnBorder : c.border}` }}>
            {data?.submitted ? "Update predictions" : "Submit predictions"}
          </button>
          <div className="mt-1.5 text-center text-[11px]" style={{ color: canSubmit ? c.faint : c.goal }}>
            {canSubmit ? "Editable until the deadline" : "Each conference needs 8 playoff teams (a 4/4 or 5/3 split)"}
          </div>
        </div>
      )}
    </div>
  );
}

function initDraft(set) {
  if (set.kind === "divisions") return {
    playoffTeams: { atlantic: [], metro: [], central: [], pacific: [] },
    divWinners: { atlantic: "", metro: "", central: "", pacific: "" },
  };
  if (set.kind === "awards") return {
    hart: ["", "", ""], norris: ["", "", ""], vezina: ["", "", ""], artRoss: ["", "", ""], rocket: ["", "", ""],
  };
  if (set.kind === "cup") return { cup: "" };
  if (set.kind === "presidents") return { presidents: "" };
  if (set.kind === "round") return { series: Object.fromEntries(R1_SERIES.map((s) => [s.id, { w: "", g: 0 }])) };
  return {};
}

/* ---------------------------- form bits ---------------------------- */
function FieldGroup({ title, hint, children, done }) {
  return (
    <div className="mb-5">
      <div className="mb-2 flex items-center gap-2">
        <h3 className="text-[14px] font-bold tracking-tight">{title}</h3>
        {done && <Check size={15} color={c.green} />}
      </div>
      {hint && <p className="mb-2 text-[12px]" style={{ color: c.faint }}>{hint}</p>}
      {children}
    </div>
  );
}

function Select({ value, onChange, disabled, placeholder, groups }) {
  return (
    <select value={value} disabled={disabled} onChange={(e) => onChange(e.target.value)}
      className="w-full rounded-lg px-3 py-2.5 text-[14px]"
      style={{ background: c.raised, color: value ? c.text : c.faint, border: `1px solid ${c.border}`, appearance: "none" }}>
      <option value="">{placeholder}</option>
      {groups.map((g) => (
        <optgroup key={g.label} label={g.label}>
          {g.options.map((o) => <option key={o.id} value={o.id} style={{ color: "#111" }}>{o.name}</option>)}
        </optgroup>
      ))}
    </select>
  );
}
const teamGroups = DIV_KEYS.map((k) => ({ label: DIVISIONS[k].label, options: DIVISIONS[k].teams }));

function PlayerInput({ value, onChange, disabled, list, placeholder }) {
  return (
    <input value={value} disabled={disabled} onChange={(e) => onChange(e.target.value)} list={list} placeholder={placeholder}
      className="w-full rounded-lg px-3 py-2.5 text-[14px]"
      style={{ background: c.raised, color: c.text, border: `1px solid ${c.border}` }} />
  );
}

function DivisionForm({ draft, setDraft, disabled }) {
  const toggleTeam = (div, id) => {
    setDraft((d) => {
      const cur = d.playoffTeams[div];
      const has = cur.includes(id);
      if (!has) {
        const conf = DIVISIONS[div].conf;
        if (cur.length >= 5) return d;                    // max 5 per division
        if (confTotal(d.playoffTeams, conf) >= 8) return d; // max 8 per conference
      }
      const next = has ? cur.filter((x) => x !== id) : [...cur, id];
      return { ...d, playoffTeams: { ...d.playoffTeams, [div]: next } };
    });
  };

  return (
    <div>
      {["east", "west"].map((cf) => {
        const conf = CONF[cf];
        const total = conf.divs.reduce((n, k) => n + draft.playoffTeams[k].length, 0);
        const ok = total === 8 && conf.divs.every((k) => { const n = draft.playoffTeams[k].length; return n >= 3 && n <= 5; });
        return (
          <FieldGroup key={cf} title={`${conf.label} playoff teams`} done={ok}
            hint="8 teams per conference — either a 4/4 split, or 5 in one division and 3 in the other.">
            {conf.divs.map((k) => {
              const d = DIVISIONS[k];
              const divLen = draft.playoffTeams[k].length;
              return (
                <div key={k} className="mb-3">
                  <div className="mb-1.5 flex items-center justify-between">
                    <span className="text-[12px] font-semibold" style={{ color: c.muted }}>{d.label}</span>
                    <span className="text-[11px]" style={{ color: c.faint, fontVariantNumeric: "tabular-nums" }}>{divLen}/5</span>
                  </div>
                  <div className="flex flex-wrap gap-1.5">
                    {d.teams.map((t) => {
                      const on = draft.playoffTeams[k].includes(t.id);
                      const capped = !on && (divLen >= 5 || total >= 8);
                      return (
                        <button key={t.id} disabled={disabled || capped} onClick={() => toggleTeam(k, t.id)}
                          className="rounded-lg px-2.5 py-1.5 text-[12px] font-semibold"
                          style={{ background: on ? c.sel : c.raised, color: on ? c.text : c.muted, border: `1px solid ${on ? c.ice : c.border}`, opacity: capped ? 0.4 : 1, fontVariantNumeric: "tabular-nums" }}>
                          {t.abbr}
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
            <div className="mt-1 flex items-center gap-1.5 text-[12px]" style={{ color: ok ? c.green : c.goal }}>
              {ok ? <Check size={14} /> : <CircleDot size={14} />}
              <span style={{ fontVariantNumeric: "tabular-nums" }}>{total}/8 selected</span>
            </div>
          </FieldGroup>
        );
      })}

      <FieldGroup title="Division winners" done={DIV_KEYS.every((k) => draft.divWinners[k])} hint="One team per division.">
        <div className="space-y-2">
          {DIV_KEYS.map((k) => (
            <div key={k}>
              <div className="mb-1 text-[12px] font-semibold" style={{ color: c.muted }}>{DIVISIONS[k].label}</div>
              <Select disabled={disabled} value={draft.divWinners[k]} placeholder="Pick a team"
                groups={[{ label: DIVISIONS[k].label, options: DIVISIONS[k].teams }]}
                onChange={(v) => setDraft((d) => ({ ...d, divWinners: { ...d.divWinners, [k]: v } }))} />
            </div>
          ))}
        </div>
      </FieldGroup>
    </div>
  );
}

function AwardsForm({ draft, setDraft, disabled }) {
  const finalists = (key, label, list, ph) => (
    <FieldGroup title={label} done={draft[key].every(Boolean)} hint="Pick 3 finalists.">
      <div className="space-y-2">
        {[0, 1, 2].map((i) => (
          <PlayerInput key={i} disabled={disabled} value={draft[key][i]} list={list} placeholder={ph}
            onChange={(v) => setDraft((d) => { const a = [...d[key]]; a[i] = v; return { ...d, [key]: a }; })} />
        ))}
      </div>
    </FieldGroup>
  );
  return (
    <div>
      <datalist id="dl-skaters">{SKATERS.map((n) => <option key={n} value={n} />)}</datalist>
      <datalist id="dl-def">{DEFENSEMEN.map((n) => <option key={n} value={n} />)}</datalist>
      <datalist id="dl-goalies">{GOALIES.map((n) => <option key={n} value={n} />)}</datalist>
      {finalists("hart", "Hart Trophy finalists", "dl-skaters", "Player name")}
      {finalists("norris", "Norris Trophy finalists", "dl-def", "Defenseman name")}
      {finalists("vezina", "Vezina Trophy finalists", "dl-goalies", "Goalie name")}
      {finalists("artRoss", "Art Ross finalists", "dl-skaters", "Player name")}
      {finalists("rocket", "Rocket Richard finalists", "dl-skaters", "Player name")}
    </div>
  );
}

function TeamPickForm({ draft, setDraft, disabled, field, title, hint }) {
  return (
    <FieldGroup title={title} done={!!draft[field]} hint={hint}>
      <Select disabled={disabled} value={draft[field]} placeholder="Pick a team" groups={teamGroups}
        onChange={(v) => setDraft({ ...draft, [field]: v })} />
    </FieldGroup>
  );
}

function RoundForm({ draft, setDraft, disabled, set }) {
  const setSeries = (id, patch) => setDraft((d) => ({ ...d, series: { ...d.series, [id]: { ...d.series[id], ...patch } } }));
  const isFinal = set?.id === "scf";
  const groups = isFinal
    ? [{ label: "Stanley Cup Final", series: R1_SERIES.slice(0, 1) }]
    : [
        { label: "Eastern Conference", series: R1_SERIES.filter((s) => teamConfById[s.a] === "east") },
        { label: "Western Conference", series: R1_SERIES.filter((s) => teamConfById[s.a] === "west") },
      ];
  return (
    <div className="space-y-5">
      <p className="text-[12px]" style={{ color: c.faint }}>For each series, tap the winner and how many games it takes.</p>
      {groups.map((g) => (
        <div key={g.label}>
          <div className="mb-2 text-[12px] font-semibold" style={{ color: c.muted }}>{g.label}</div>
          <div className="space-y-3">
            {g.series.map((s) => {
              const pick = draft.series?.[s.id] || { w: "", g: 0 };
              return (
                <div key={s.id} className="rounded-xl p-3" style={{ background: c.surface, border: `1px solid ${c.border}` }}>
                  <div className="mb-2 flex gap-2">
                    {[s.a, s.b].map((tid) => {
                      const on = pick.w === tid;
                      return (
                        <button key={tid} disabled={disabled} onClick={() => setSeries(s.id, { w: tid })}
                          className="flex-1 rounded-lg py-2 text-[13px] font-bold"
                          style={{ background: on ? c.sel : c.raised, color: c.text, border: `1px solid ${on ? c.ice : c.border}` }}>
                          {teamById[tid].name}
                        </button>
                      );
                    })}
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-[12px]" style={{ color: c.muted }}>in</span>
                    {[4, 5, 6, 7].map((g2) => {
                      const on = pick.g === g2;
                      return (
                        <button key={g2} disabled={disabled} onClick={() => setSeries(s.id, { g: g2 })}
                          className="h-9 w-9 rounded-lg text-[13px] font-bold"
                          style={{ background: on ? c.sel : c.raised, color: c.text, border: `1px solid ${on ? c.ice : c.border}`, fontVariantNumeric: "tabular-nums" }}>
                          {g2}
                        </button>
                      );
                    })}
                    <span className="text-[12px]" style={{ color: c.muted }}>games</span>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}

/* --------------------------- bottom nav --------------------------- */
function BottomNav({ tab, setTab }) {
  const items = [
    { id: "predict", label: "Predict", icon: ListChecks },
    { id: "board", label: "Leaderboard", icon: Medal },
    { id: "compare", label: "Compare", icon: Users },
  ];
  return (
    <nav className="shrink-0 z-30 flex" style={{ background: c.surface, borderTop: `1px solid ${c.border}` }}>
      {items.map((it) => {
        const on = tab === it.id;
        const Icon = it.icon;
        return (
          <button key={it.id} onClick={() => setTab(it.id)} className="flex flex-1 flex-col items-center gap-1 py-2.5">
            <Icon size={20} color={on ? c.ice : c.muted} />
            <span className="text-[11px] font-semibold" style={{ color: on ? c.ice : c.muted }}>{it.label}</span>
          </button>
        );
      })}
    </nav>
  );
}
