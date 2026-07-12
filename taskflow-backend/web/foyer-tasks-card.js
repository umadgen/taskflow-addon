// ============================================================================
//  Foyer Tasks Card + Progress Card — custom Lovelace cards pour Home Assistant
//  Synchronisées en temps réel avec l'app FOYER (Taskflow) via MQTT.
//
//  Ressource Lovelace : /local/foyer-tasks-card.js  (type Module)
//
//  Cards disponibles :
//    type: custom:foyer-tasks-card
//      foyer_sensor: sensor.foyer_snapshot   # requis
//      title: Tâches du foyer               # optionnel
//      show_done: false
//      show_upcoming: false
//      history_count: 10
//
//    type: custom:foyer-progress-card
//      foyer_sensor: sensor.foyer_snapshot   # requis
//      title: Progression du foyer          # optionnel
// ============================================================================

const FOYER_VERSION = '1.8.1';

const FOYER_CAT_ICONS   = { maison: '🏠', courses: '🛒', animaux: '🐾' };
const FOYER_CAT_COLORS  = { maison: '#26a69a', courses: '#ff7043', animaux: '#ab47bc' };
const FOYER_TONE_COLORS = {
  rose: '#ec4899', peach: '#ff7043', amber: '#ffa000',
  mint: '#00897b', sky: '#039be5', lilac: '#7e57c2',
};

function foyerEsc(s) {
  return String(s ?? '').replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}
function foyerToday() {
  // Local calendar date (not UTC) — France is always ahead of UTC, so a
  // UTC-based "today" lags behind the real local day for 1-2h after midnight.
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
function foyerLocalDateOf(iso) {
  // History "at" values are UTC instants (see foyerParseAt); convert to the
  // local calendar day before comparing against a local "today"/"weekStart".
  if (!iso) return '';
  const d = foyerParseAt(iso);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
function foyerParseAt(iso) {
  // Legacy entries (pre-1.6.7) were stored without a timezone marker (e.g.
  // "2026-07-08T18:13"), which JS parses as *local* time instead of UTC.
  // Treat any datetime string lacking an explicit offset as UTC.
  if (typeof iso === 'string' && iso.includes('T') && !/[Zz]|[+-]\d{2}:?\d{2}$/.test(iso)) {
    return new Date(iso + 'Z');
  }
  return new Date(iso);
}
function foyerFormatAt(iso) {
  const d = foyerParseAt(iso), now = new Date(), opts = { day:'numeric', month:'long' };
  const today = now.toLocaleDateString('fr-FR', opts);
  const yest  = new Date(now - 864e5).toLocaleDateString('fr-FR', opts);
  const ds    = d.toLocaleDateString('fr-FR', opts);
  const ts    = d.toLocaleTimeString('fr-FR', { hour:'2-digit', minute:'2-digit' });
  return ds === today ? `Aujourd'hui à ${ts}` : ds === yest ? `Hier à ${ts}` : `${ds} à ${ts}`;
}

// ─────────────────────────────────────────────────────────────────────────────
//  FoyerTasksCard
// ─────────────────────────────────────────────────────────────────────────────
class FoyerTasksCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this._hass          = null;
    this._config        = null;
    this._pendingTask   = null;
    this._pendingAction = null;
    this._seq           = null;
    this._rendered      = false;
    this._today         = null;
    this._dayTimer      = null;
    this._criticalTimer = null;
  }

  connectedCallback() {
    // HA/Lovelace can detach and reattach this element (view switches, dashboard
    // relayout) without recreating it, which would otherwise leave the day-change
    // poller dead forever since disconnectedCallback tears it down below.
    this._startDayTimer();
    this._startCriticalTimer();
  }

  disconnectedCallback() {
    if (this._dayTimer) { clearInterval(this._dayTimer); this._dayTimer = null; }
    if (this._criticalTimer) { clearInterval(this._criticalTimer); this._criticalTimer = null; }
  }

  _startDayTimer() {
    if (this._dayTimer || !this._rendered) return;
    this._dayTimer = setInterval(() => {
      const d = foyerToday();
      if (d !== this._today) { this._today = d; this._renderTasks(); }
    }, 60000);
  }

  // Critical tasks need to flip into "alert" state the instant their due
  // time is reached, which is finer-grained than the daily late bucketing —
  // so poll independently and just toggle a class on the already-rendered
  // rows rather than forcing a full re-render.
  _startCriticalTimer() {
    if (this._criticalTimer || !this._rendered) return;
    this._updateCriticalAlerts();
    this._criticalTimer = setInterval(() => this._updateCriticalAlerts(), 20000);
  }

  _updateCriticalAlerts() {
    const tasks = this._getData()?.tasks ?? [];
    const now = Date.now();
    this.shadowRoot.querySelectorAll('.task-row[data-critical]').forEach(row => {
      const task = tasks.find(t => t.id === row.dataset.task);
      const due  = task ? new Date(task.due).getTime() : NaN;
      row.classList.toggle('critical-alert', !!task && !task.done && due <= now);
    });
  }

  setConfig(config) {
    if (!config.foyer_sensor) throw new Error('foyer-tasks-card : "foyer_sensor" est requis.');
    this._config = {
      title:         'Tâches du foyer',
      show_done:     false,
      show_upcoming: false,
      history_count: 10,
      ...config,
    };
    this._rendered = false;
    if (this._hass) this._init();
  }

  set hass(hass) {
    this._hass = hass;
    const seq = hass.states[this._config?.foyer_sensor]?.state;
    if (!this._rendered) { this._init(); return; }
    if (seq !== this._seq) { this._seq = seq; this._renderTasks(); }
  }

  // ── Data ──────────────────────────────────────────────────────────────────
  _getData() {
    const state = this._hass?.states[this._config.foyer_sensor];
    if (!state || state.state === 'unavailable' || state.state === 'unknown') return null;
    return {
      tasks:   state.attributes.tasks   ?? [],
      members: state.attributes.members ?? [],
      history: state.attributes.history ?? [],
    };
  }

  // ── Full render (once) ────────────────────────────────────────────────────
  _init() {
    this.shadowRoot.innerHTML = `
      <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        :host { display: block; }

        /* ── Card shell ── */
        .card {
          background: var(--card-background-color);
          border-radius: var(--ha-card-border-radius, 12px);
          box-shadow: var(--ha-card-box-shadow, 0 2px 8px rgba(0,0,0,.1));
          overflow: hidden;
        }

        /* ── Header ── */
        .card-header {
          padding: 18px 20px 14px;
          border-bottom: 1px solid var(--divider-color, rgba(0,0,0,.08));
        }
        .header-row {
          display: flex;
          align-items: center;
          justify-content: space-between;
          margin-bottom: 10px;
          gap: 12px;
        }
        .card-title {
          font-size: 16px;
          font-weight: 700;
          color: var(--primary-text-color);
        }
        .late-pill {
          font-size: 11px;
          font-weight: 700;
          background: var(--error-color, #f44336);
          color: #fff;
          padding: 3px 10px;
          border-radius: 12px;
          white-space: nowrap;
          flex-shrink: 0;
          letter-spacing: .02em;
        }
        .late-pill.hidden { display: none; }

        .progress-track {
          height: 4px;
          background: var(--divider-color, rgba(0,0,0,.1));
          border-radius: 2px;
          overflow: hidden;
          margin-bottom: 6px;
        }
        .progress-fill {
          height: 100%;
          border-radius: 2px;
          background: var(--primary-color, #03a9f4);
          transition: width .4s ease;
        }
        .progress-fill.done { background: var(--success-color, #4caf50); }
        .progress-label {
          font-size: 11px;
          color: var(--secondary-text-color);
          letter-spacing: .01em;
        }

        /* ── Section label ── */
        .section-label {
          display: flex;
          align-items: center;
          gap: 10px;
          font-size: 10px;
          font-weight: 700;
          text-transform: uppercase;
          letter-spacing: .08em;
          color: var(--secondary-text-color);
          padding: 14px 20px 6px;
        }
        .section-label::after {
          content: '';
          flex: 1;
          height: 1px;
          background: var(--divider-color, rgba(0,0,0,.08));
        }

        /* ── Task row ── */
        .task-row {
          position: relative;
          display: flex;
          align-items: center;
          gap: 12px;
          padding: 12px 16px 12px 0;
          border-bottom: 1px solid var(--divider-color, rgba(0,0,0,.05));
          cursor: pointer;
          user-select: none;
          -webkit-tap-highlight-color: transparent;
          transition: background .12s;
          animation: fadeSlide .15s ease both;
        }
        @keyframes fadeSlide {
          from { opacity: 0; transform: translateY(4px); }
          to   { opacity: 1; transform: translateY(0); }
        }
        .task-row:last-child { border-bottom: none; }
        .task-row:active { background: var(--secondary-background-color, rgba(0,0,0,.03)); }
        .task-row.done { opacity: .4; pointer-events: none; }

        /* ── Critical task alert (due time reached/passed) ── */
        .critical-dot { position: absolute; top: 6px; right: 10px; width: 10px; height: 10px; display: none; }
        .task-row.critical-alert .critical-dot { display: block; }
        .critical-dot::before, .critical-dot::after {
          content: '';
          position: absolute;
          inset: 0;
          border-radius: 50%;
          background: var(--error-color, #f44336);
        }
        .critical-dot::after {
          animation: criticalPing 1.6s cubic-bezier(0,0,.2,1) infinite;
        }
        @keyframes criticalPing {
          0%           { transform: scale(1);   opacity: .7; }
          75%, 100%    { transform: scale(2.4);  opacity: 0;  }
        }
        .task-row.critical-alert {
          border-radius: 10px;
          animation: fadeSlide .15s ease both, criticalGlow 1.6s ease-in-out infinite;
        }
        @keyframes criticalGlow {
          0%, 100% { box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--error-color, #f44336) 25%, transparent); }
          50%      { box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--error-color, #f44336) 70%, transparent); }
        }
        @media (prefers-reduced-motion: reduce) {
          .critical-dot::after { animation: none; opacity: .7; }
          .task-row.critical-alert { animation: fadeSlide .15s ease both; box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--error-color, #f44336) 45%, transparent); }
        }

        /* ── Upcoming compact rows ── */
        .upcoming-list { padding: 0 16px 6px 12px; }
        .upcoming-row {
          display: flex;
          align-items: center;
          gap: 8px;
          padding: 6px 0;
          border-bottom: 1px solid var(--divider-color, rgba(0,0,0,.04));
          opacity: .5;
          user-select: none;
        }
        .upcoming-row:last-child { border-bottom: none; }
        .upcoming-icon { font-size: 14px; flex-shrink: 0; width: 18px; text-align: center; }
        .upcoming-name {
          flex: 1;
          font-size: 12px;
          font-weight: 400;
          color: var(--secondary-text-color);
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .upcoming-due {
          font-size: 10px;
          font-weight: 600;
          color: var(--secondary-text-color);
          background: var(--secondary-background-color, rgba(0,0,0,.06));
          padding: 2px 7px;
          border-radius: 8px;
          white-space: nowrap;
          flex-shrink: 0;
          letter-spacing: .01em;
        }

        .cat-bar {
          width: 3px;
          align-self: stretch;
          border-radius: 0 2px 2px 0;
          background: var(--cat-color, #888);
          flex-shrink: 0;
          margin-right: 2px;
        }
        .task-icon { font-size: 20px; width: 26px; text-align: center; flex-shrink: 0; }
        .task-body { flex: 1; min-width: 0; }
        .task-name {
          display: block;
          font-size: 14px;
          font-weight: 500;
          color: var(--primary-text-color);
          line-height: 1.4;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .task-name.striked {
          text-decoration: line-through;
          color: var(--secondary-text-color);
        }
        .task-meta  { display: block; font-size: 12px; color: var(--secondary-text-color); margin-top: 2px; }
        .task-recur { display: block; font-size: 11px; color: var(--secondary-text-color); opacity: .65; font-style: italic; margin-top: 1px; }
        .task-note  { display: block; font-size: 12px; color: var(--secondary-text-color); margin-top: 2px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

        /* ── Filter bar ── */
        .filter-row { display: flex; gap: 6px; overflow-x: auto; scrollbar-width: none; padding: 0 16px 10px; -webkit-tap-highlight-color: transparent; }
        .filter-row::-webkit-scrollbar { display: none; }
        .filter-chip {
          display: flex; align-items: center; gap: 5px;
          padding: 4px 10px 4px 5px; border-radius: 20px;
          border: 2px solid var(--divider-color, rgba(0,0,0,.1));
          font-size: 12px; font-weight: 600;
          color: var(--secondary-text-color); background: transparent;
          cursor: pointer; white-space: nowrap; flex-shrink: 0;
          -webkit-tap-highlight-color: transparent;
          transition: border-color .1s, background .1s, color .1s;
        }
        .filter-chip.active { background: var(--primary-color, #03a9f4); border-color: var(--primary-color, #03a9f4); color: #fff; }
        .filter-chip .f-av {
          width: 18px; height: 18px; border-radius: 50%;
          display: flex; align-items: center; justify-content: center;
          font-size: 9px; font-weight: 700; color: #fff; flex-shrink: 0;
        }
        .filter-chip.active .f-av { background: rgba(255,255,255,.35); }
        .filter-chip:not(.active) .f-av { background: var(--divider-color, rgba(0,0,0,.15)); }

        .late-badge {
          display: inline-block;
          font-size: 9px;
          font-weight: 700;
          text-transform: uppercase;
          letter-spacing: .04em;
          background: var(--error-color, #f44336);
          color: #fff;
          padding: 1px 5px;
          border-radius: 4px;
          margin-left: 5px;
          vertical-align: middle;
        }
        .task-chevron {
          font-size: 18px;
          color: var(--secondary-text-color);
          flex-shrink: 0;
          opacity: .4;
        }
        .task-row.done .task-chevron {
          color: var(--success-color, #4caf50);
          opacity: 1;
          font-size: 15px;
          font-weight: 700;
        }

        /* ── Empty / offline ── */
        .empty-state, .offline-state {
          padding: 36px 20px;
          text-align: center;
          font-size: 14px;
          color: var(--secondary-text-color);
          line-height: 1.6;
        }
        .empty-icon { font-size: 40px; display: block; margin-bottom: 10px; }
        .offline-state { color: var(--warning-color, #ff9800); }

        /* ── Modal overlay ── */
        .modal-overlay {
          position: fixed;
          inset: 0;
          background: rgba(0,0,0,.45);
          z-index: 9999;
          display: flex;
          align-items: flex-end;
          justify-content: center;
          opacity: 0;
          pointer-events: none;
          transition: opacity .18s ease;
        }
        @media (min-width: 480px) { .modal-overlay { align-items: center; } }
        .modal-overlay.open { opacity: 1; pointer-events: auto; }

        .modal {
          background: var(--card-background-color);
          border-radius: 24px 24px 0 0;
          padding: 12px 24px 32px;
          width: 100%;
          max-width: 480px;
          box-shadow: 0 -8px 32px rgba(0,0,0,.2);
          transform: translateY(32px);
          transition: transform .2s cubic-bezier(.34,1.56,.64,1);
        }
        @media (min-width: 480px) {
          .modal { border-radius: 24px; padding-bottom: 24px; transform: scale(.95); }
          .modal-overlay.open .modal { transform: scale(1); }
        }
        .modal-overlay.open .modal { transform: translateY(0); }

        .modal-drag {
          width: 36px; height: 4px;
          background: var(--divider-color, rgba(0,0,0,.15));
          border-radius: 2px;
          margin: 0 auto 20px;
        }
        @media (min-width: 480px) { .modal-drag { display: none; } }

        .modal-eyebrow {
          font-size: 11px;
          font-weight: 700;
          text-transform: uppercase;
          letter-spacing: .07em;
          color: var(--secondary-text-color);
          margin-bottom: 6px;
        }
        .modal-task-name {
          font-size: 20px;
          font-weight: 700;
          color: var(--primary-text-color);
          margin-bottom: 24px;
          line-height: 1.3;
        }

        /* ── Person buttons ── */
        .people-row {
          display: flex;
          gap: 10px;
          margin-bottom: 14px;
          flex-wrap: wrap;
        }
        .person-btn {
          flex: 1;
          min-width: 80px;
          background: transparent;
          border: 2px solid var(--c, #888);
          border-radius: 16px;
          padding: 14px 8px;
          cursor: pointer;
          display: flex;
          flex-direction: column;
          align-items: center;
          gap: 8px;
          -webkit-tap-highlight-color: transparent;
          transition: background .12s, transform .1s;
        }
        .person-btn:active {
          background: color-mix(in srgb, var(--c) 14%, transparent);
          transform: scale(.97);
        }
        .person-avatar {
          width: 48px; height: 48px;
          border-radius: 50%;
          background: var(--c, #888);
          color: #fff;
          font-size: 20px;
          font-weight: 700;
          display: flex;
          align-items: center;
          justify-content: center;
        }
        .person-name { font-size: 13px; font-weight: 600; color: var(--c, #888); }

        /* ── Cancel ── */
        .modal-cancel {
          width: 100%;
          padding: 12px;
          background: var(--secondary-background-color, rgba(0,0,0,.05));
          border: none;
          border-radius: 12px;
          color: var(--secondary-text-color);
          font-size: 14px;
          cursor: pointer;
          transition: background .1s;
        }
        .modal-cancel:active { background: var(--divider-color, rgba(0,0,0,.1)); }

        /* ── Add button ── */
        .add-btn {
          width: 28px; height: 28px;
          border-radius: 50%;
          border: none;
          background: var(--primary-color, #03a9f4);
          color: #fff;
          font-size: 20px;
          line-height: 1;
          cursor: pointer;
          display: flex;
          align-items: center;
          justify-content: center;
          flex-shrink: 0;
          margin-left: auto;
          padding: 0;
          transition: transform .1s, opacity .1s;
          -webkit-tap-highlight-color: transparent;
        }
        .add-btn:active { transform: scale(.9); opacity: .8; }

        /* ── Create modal ── */
        .create-overlay {
          position: fixed;
          inset: 0;
          background: rgba(0,0,0,.45);
          z-index: 9999;
          display: flex;
          align-items: flex-end;
          justify-content: center;
          opacity: 0;
          pointer-events: none;
          transition: opacity .18s ease;
        }
        @media (min-width: 480px) { .create-overlay { align-items: center; } }
        .create-overlay.open { opacity: 1; pointer-events: auto; }
        .create-modal {
          background: var(--card-background-color);
          border-radius: 24px 24px 0 0;
          padding: 12px 24px 32px;
          width: 100%;
          max-width: 480px;
          box-shadow: 0 -8px 32px rgba(0,0,0,.2);
          transform: translateY(32px);
          transition: transform .2s cubic-bezier(.34,1.56,.64,1);
          max-height: 92vh;
          overflow-y: auto;
        }
        @media (min-width: 480px) {
          .create-modal { border-radius: 24px; padding-bottom: 24px; transform: scale(.95); }
          .create-overlay.open .create-modal { transform: scale(1); }
        }
        .create-overlay.open .create-modal { transform: translateY(0); }

        .create-title-input {
          width: 100%;
          border: none;
          border-bottom: 2px solid var(--primary-color, #03a9f4);
          background: transparent;
          font-size: 18px;
          font-weight: 600;
          color: var(--primary-text-color);
          padding: 8px 0 10px;
          outline: none;
          margin-bottom: 22px;
          box-sizing: border-box;
        }
        .create-title-input::placeholder { color: var(--secondary-text-color); font-weight: 400; }
        .create-field-label {
          display: block;
          font-size: 10px;
          font-weight: 700;
          text-transform: uppercase;
          letter-spacing: .07em;
          color: var(--secondary-text-color);
          margin-bottom: 8px;
        }
        .chip-row { display: flex; gap: 8px; margin-bottom: 20px; flex-wrap: wrap; }
        .chip {
          padding: 6px 14px;
          border-radius: 20px;
          border: 2px solid var(--divider-color, rgba(0,0,0,.12));
          background: transparent;
          font-size: 13px;
          cursor: pointer;
          color: var(--secondary-text-color);
          transition: all .12s;
          -webkit-tap-highlight-color: transparent;
        }
        .chip.active {
          border-color: var(--primary-color, #03a9f4);
          background: color-mix(in srgb, var(--primary-color, #03a9f4) 12%, transparent);
          color: var(--primary-text-color);
          font-weight: 600;
        }
        .chip.critical-chip.active {
          border-color: var(--error-color, #f44336);
          background: color-mix(in srgb, var(--error-color, #f44336) 14%, transparent);
          color: var(--primary-text-color);
        }
        .wd-row { display: flex; gap: 5px; margin-bottom: 20px; }
        .wd-btn {
          flex: 1;
          height: 36px;
          border-radius: 8px;
          border: 2px solid var(--divider-color, rgba(0,0,0,.12));
          background: transparent;
          font-size: 11px;
          font-weight: 700;
          cursor: pointer;
          color: var(--secondary-text-color);
          transition: all .12s;
          padding: 0;
          -webkit-tap-highlight-color: transparent;
        }
        .wd-btn.active {
          border-color: var(--primary-color, #03a9f4);
          background: color-mix(in srgb, var(--primary-color, #03a9f4) 15%, transparent);
          color: var(--primary-color, #03a9f4);
        }
        .monthday-row {
          display: flex;
          align-items: center;
          gap: 10px;
          margin-bottom: 20px;
          font-size: 14px;
          color: var(--secondary-text-color);
        }
        .monthday-input {
          width: 60px;
          border: 2px solid var(--divider-color, rgba(0,0,0,.1));
          border-radius: 8px;
          padding: 8px;
          font-size: 15px;
          text-align: center;
          color: var(--primary-text-color);
          background: var(--secondary-background-color, rgba(0,0,0,.03));
          outline: none;
        }
        .create-time-input {
          width: 100%;
          border: 2px solid var(--divider-color, rgba(0,0,0,.1));
          border-radius: 8px;
          background: var(--secondary-background-color, rgba(0,0,0,.03));
          padding: 10px 12px;
          font-size: 15px;
          color: var(--primary-text-color);
          outline: none;
          margin-bottom: 24px;
          box-sizing: border-box;
        }
        .date-row { margin-bottom: 20px; }
        .create-date-input {
          width: 100%;
          border: 2px solid var(--divider-color, rgba(0,0,0,.1));
          border-radius: 8px;
          background: var(--secondary-background-color, rgba(0,0,0,.03));
          padding: 10px 12px;
          font-size: 15px;
          color: var(--primary-text-color);
          outline: none;
          box-sizing: border-box;
        }
        .create-submit {
          width: 100%;
          padding: 14px;
          background: var(--primary-color, #03a9f4);
          border: none;
          border-radius: 12px;
          color: #fff;
          font-size: 15px;
          font-weight: 700;
          cursor: pointer;
          margin-bottom: 10px;
          transition: opacity .1s;
        }
        .create-submit:active { opacity: .85; }
        .create-submit:disabled { opacity: .45; cursor: default; }
        .hidden-field { display: none; }

        /* ── Assignee chips (in create form) ── */
        .assignee-chips { display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 20px; }
        .assignee-chip {
          display: flex; align-items: center; gap: 6px;
          padding: 5px 12px 5px 5px;
          border-radius: 20px;
          border: 2px solid var(--divider-color, rgba(0,0,0,.1));
          cursor: pointer; font-size: 13px; font-weight: 500;
          color: var(--secondary-text-color); background: transparent;
          -webkit-tap-highlight-color: transparent; transition: border-color .1s, color .1s;
        }
        .assignee-chip.active { border-color: var(--primary-color, #03a9f4); color: var(--primary-text-color); }
        .assignee-avatar-sm {
          width: 22px; height: 22px; border-radius: 50%;
          display: flex; align-items: center; justify-content: center;
          font-size: 10px; font-weight: 700; color: #fff; flex-shrink: 0;
        }

        /* ── Members overlay ── */
        .members-btn {
          width: 28px; height: 28px;
          border-radius: 50%;
          border: none;
          background: var(--secondary-background-color, rgba(0,0,0,.07));
          color: var(--primary-text-color);
          font-size: 15px;
          line-height: 1;
          cursor: pointer;
          display: flex; align-items: center; justify-content: center;
          flex-shrink: 0;
          padding: 0;
          -webkit-tap-highlight-color: transparent;
        }
        .members-overlay {
          position: fixed; inset: 0;
          background: rgba(0,0,0,.45);
          z-index: 9999;
          display: flex; align-items: flex-end; justify-content: center;
          opacity: 0; pointer-events: none;
          transition: opacity .18s ease;
        }
        .members-overlay.open { opacity: 1; pointer-events: auto; }
        .members-modal {
          background: var(--card-background-color);
          border-radius: 24px 24px 0 0;
          padding: 12px 16px 36px;
          width: 100%; max-width: 480px;
          box-shadow: 0 -8px 32px rgba(0,0,0,.2);
          transform: translateY(32px);
          transition: transform .2s cubic-bezier(.34,1.56,.64,1);
        }
        .members-overlay.open .members-modal { transform: translateY(0); }
        .members-modal-title {
          font-size: 16px; font-weight: 700;
          color: var(--primary-text-color);
          padding: 4px 4px 16px;
          display: flex; align-items: center; justify-content: space-between;
        }
        .members-add-btn {
          width: 28px; height: 28px;
          border-radius: 50%; border: none;
          background: var(--primary-color, #03a9f4);
          color: #fff; font-size: 20px; line-height: 1;
          cursor: pointer; display: flex; align-items: center; justify-content: center;
          padding: 0; flex-shrink: 0;
        }
        .member-row {
          display: flex; align-items: center; gap: 12px;
          padding: 10px 4px;
          border-bottom: 1px solid var(--divider-color, rgba(0,0,0,.06));
          cursor: pointer;
          -webkit-tap-highlight-color: transparent;
        }
        .member-row:last-child { border-bottom: none; }
        .member-avatar {
          width: 38px; height: 38px; border-radius: 50%;
          display: flex; align-items: center; justify-content: center;
          font-size: 16px; font-weight: 700; color: #fff;
          flex-shrink: 0;
        }
        .member-info { flex: 1; min-width: 0; }
        .member-name { font-size: 15px; font-weight: 600; color: var(--primary-text-color); }
        .member-delete {
          width: 32px; height: 32px; border-radius: 50%; border: none;
          background: transparent; color: var(--error-color, #f44336);
          font-size: 17px; cursor: pointer; display: flex; align-items: center; justify-content: center;
          flex-shrink: 0; -webkit-tap-highlight-color: transparent;
        }

        /* Member form */
        .mform-overlay {
          position: fixed; inset: 0;
          background: rgba(0,0,0,.45);
          z-index: 10000;
          display: flex; align-items: flex-end; justify-content: center;
          opacity: 0; pointer-events: none;
          transition: opacity .18s ease;
        }
        .mform-overlay.open { opacity: 1; pointer-events: auto; }
        .mform-modal {
          background: var(--card-background-color);
          border-radius: 24px 24px 0 0;
          padding: 12px 16px 36px;
          width: 100%; max-width: 480px;
          box-shadow: 0 -8px 32px rgba(0,0,0,.2);
          transform: translateY(32px);
          transition: transform .2s cubic-bezier(.34,1.56,.64,1);
        }
        .mform-overlay.open .mform-modal { transform: translateY(0); }
        .mform-title {
          font-size: 16px; font-weight: 700;
          color: var(--primary-text-color);
          padding: 4px 4px 16px;
        }
        .mform-label { font-size: 12px; font-weight: 600; color: var(--secondary-text-color); text-transform: uppercase; letter-spacing: .06em; margin-bottom: 8px; display: block; }
        .mform-input {
          width: 100%; border: 2px solid var(--divider-color, rgba(0,0,0,.1));
          border-radius: 8px; background: var(--secondary-background-color, rgba(0,0,0,.03));
          padding: 10px 12px; font-size: 15px; color: var(--primary-text-color);
          outline: none; margin-bottom: 18px; box-sizing: border-box;
        }
        .tone-row { display: flex; gap: 10px; margin-bottom: 24px; flex-wrap: wrap; }
        .tone-swatch {
          width: 36px; height: 36px; border-radius: 50%; cursor: pointer;
          border: 3px solid transparent;
          transition: transform .1s, border-color .1s;
        }
        .tone-swatch.active { border-color: var(--primary-text-color); transform: scale(1.15); }
        .mform-submit {
          width: 100%; padding: 14px;
          background: var(--primary-color, #03a9f4); border: none; border-radius: 12px;
          color: #fff; font-size: 15px; font-weight: 700; cursor: pointer;
          margin-bottom: 10px; transition: opacity .1s;
        }
        .mform-submit:disabled { opacity: .45; cursor: default; }

        /* ── Context menu ── */
        .ctx-overlay {
          position: fixed;
          inset: 0;
          background: rgba(0,0,0,.45);
          z-index: 9999;
          display: flex;
          align-items: flex-end;
          justify-content: center;
          opacity: 0;
          pointer-events: none;
          transition: opacity .18s ease;
        }
        @media (min-width: 480px) { .ctx-overlay { align-items: center; } }
        .ctx-overlay.open { opacity: 1; pointer-events: auto; }
        .ctx-modal {
          background: var(--card-background-color);
          border-radius: 24px 24px 0 0;
          padding: 12px 16px 32px;
          width: 100%;
          max-width: 480px;
          box-shadow: 0 -8px 32px rgba(0,0,0,.2);
          transform: translateY(32px);
          transition: transform .2s cubic-bezier(.34,1.56,.64,1);
        }
        @media (min-width: 480px) {
          .ctx-modal { border-radius: 24px; transform: scale(.95); padding-bottom: 24px; }
          .ctx-overlay.open .ctx-modal { transform: scale(1); }
        }
        .ctx-overlay.open .ctx-modal { transform: translateY(0); }
        .ctx-task-name {
          font-size: 15px;
          font-weight: 700;
          color: var(--primary-text-color);
          padding: 4px 4px 16px;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .ctx-btn {
          display: flex;
          align-items: center;
          gap: 14px;
          width: 100%;
          padding: 14px 12px;
          background: transparent;
          border: none;
          border-radius: 12px;
          font-size: 15px;
          font-weight: 500;
          color: var(--primary-text-color);
          cursor: pointer;
          text-align: left;
          transition: background .1s;
          -webkit-tap-highlight-color: transparent;
        }
        .ctx-btn:active { background: var(--secondary-background-color, rgba(0,0,0,.05)); }
        .ctx-btn .ctx-icon { font-size: 20px; width: 28px; text-align: center; flex-shrink: 0; }
        .ctx-btn.danger { color: var(--error-color, #f44336); }
        .ctx-divider { height: 1px; background: var(--divider-color, rgba(0,0,0,.08)); margin: 4px 0; }

        /* Long-press feedback */
        .task-row.pressing { background: var(--secondary-background-color, rgba(0,0,0,.04)); }
      </style>

      <ha-card class="card">
        <div class="card-header">
          <div class="header-row">
            <div class="card-title">${this._esc(this._config.title)}</div>
            <button class="members-btn" title="Gérer les membres">👥</button>
            <button class="add-btn" title="Ajouter une tâche">+</button>
            <div class="late-pill hidden"></div>
          </div>
          <div class="progress-track"><div class="progress-fill" style="width:0%"></div></div>
          <div class="progress-label">Chargement…</div>
        </div>
        <div class="filter-row"></div>
        <div class="task-list"></div>
      </ha-card>

      <div class="modal-overlay">
        <div class="modal">
          <div class="modal-drag"></div>
          <div class="modal-eyebrow">Qui a fait ça ?</div>
          <div class="modal-task-name"></div>
          <div class="people-row"></div>
          <button class="modal-cancel">Annuler</button>
        </div>
      </div>

      <div class="create-overlay">
        <div class="create-modal">
          <div class="modal-drag"></div>
          <input class="create-title-input" type="text" placeholder="Nom de la tâche…" autocomplete="off" />

          <span class="create-field-label">Catégorie</span>
          <div class="chip-row">
            <button class="chip active" data-cat="maison">🏠 Maison</button>
            <button class="chip" data-cat="courses">🛒 Courses</button>
            <button class="chip" data-cat="animaux">🐾 Animaux</button>
          </div>

          <span class="create-field-label">Récurrence</span>
          <div class="chip-row">
            <button class="chip" data-repeat="once">Une fois</button>
            <button class="chip" data-repeat="jour">Quotidien</button>
            <button class="chip active" data-repeat="semaine">Hebdo</button>
            <button class="chip" data-repeat="mois">Mensuel</button>
          </div>

          <div class="wd-row">
            <button class="wd-btn" data-wd="0">L</button>
            <button class="wd-btn" data-wd="1">Ma</button>
            <button class="wd-btn" data-wd="2">Me</button>
            <button class="wd-btn" data-wd="3">J</button>
            <button class="wd-btn" data-wd="4">V</button>
            <button class="wd-btn" data-wd="5">S</button>
            <button class="wd-btn" data-wd="6">D</button>
          </div>

          <div class="monthday-row hidden-field">
            <span>Le</span>
            <input class="monthday-input" type="number" min="1" max="31" value="1" />
            <span>de chaque mois</span>
          </div>

          <div class="date-row hidden-field">
            <span class="create-field-label">Date</span>
            <input class="create-date-input" type="date" />
          </div>

          <span class="create-field-label">Heure</span>
          <input class="create-time-input" type="time" value="18:00" />

          <div class="chip-row">
            <button class="chip critical-chip" data-critical="1"><span class="ctx-icon">⚠️</span>Tâche critique</button>
          </div>

          <span class="create-field-label">Assigné à</span>
          <div class="assignee-chips"></div>

          <span class="create-field-label">Note <span style="font-weight:400;text-transform:none;letter-spacing:0">(optionnel)</span></span>
          <input class="create-note-input" type="text" placeholder="ex : lait entier, pain de campagne…" autocomplete="off" style="width:100%;border:2px solid var(--divider-color,rgba(0,0,0,.1));border-radius:8px;background:var(--secondary-background-color,rgba(0,0,0,.03));padding:10px 12px;font-size:15px;color:var(--primary-text-color);outline:none;margin-bottom:20px;box-sizing:border-box" />

          <button class="create-submit">Ajouter la tâche</button>
          <button class="modal-cancel create-cancel">Annuler</button>
        </div>
      </div>

      <div class="ctx-overlay">
        <div class="ctx-modal">
          <div class="modal-drag"></div>
          <div class="ctx-task-name"></div>
          <button class="ctx-btn ctx-postpone"><span class="ctx-icon">⏩</span>Décaler à demain</button>
          <button class="ctx-btn ctx-skip"><span class="ctx-icon">⏭</span>Ignorer cette occurrence</button>
          <button class="ctx-btn ctx-edit"><span class="ctx-icon">✏️</span>Modifier la tâche</button>
          <div class="ctx-divider"></div>
          <button class="ctx-btn ctx-delete danger"><span class="ctx-icon">🗑</span>Supprimer définitivement</button>
          <div style="height:10px"></div>
          <button class="modal-cancel ctx-cancel">Annuler</button>
        </div>
      </div>

      <div class="members-overlay">
        <div class="members-modal">
          <div class="modal-drag"></div>
          <div class="members-modal-title">
            <span>Membres</span>
            <button class="members-add-btn" title="Ajouter un membre">+</button>
          </div>
          <div class="members-list"></div>
          <div style="height:10px"></div>
          <button class="modal-cancel members-close">Fermer</button>
        </div>
      </div>

      <div class="mform-overlay">
        <div class="mform-modal">
          <div class="modal-drag"></div>
          <div class="mform-title">Nouveau membre</div>
          <span class="mform-label">Prénom</span>
          <input class="mform-input mform-name" type="text" placeholder="ex : Camille" autocomplete="off" />
          <span class="mform-label">Initiale</span>
          <input class="mform-input mform-initial" type="text" maxlength="2" placeholder="ex : C" autocomplete="off" />
          <span class="mform-label">Couleur</span>
          <div class="tone-row">
            <div class="tone-swatch active" data-tone="sky"    style="background:#0ea5e9"></div>
            <div class="tone-swatch"        data-tone="rose"   style="background:#ec4899"></div>
            <div class="tone-swatch"        data-tone="peach"  style="background:#fb923c"></div>
            <div class="tone-swatch"        data-tone="amber"  style="background:#f59e0b"></div>
            <div class="tone-swatch"        data-tone="mint"   style="background:#10b981"></div>
            <div class="tone-swatch"        data-tone="lilac"  style="background:#a78bfa"></div>
          </div>
          <button class="mform-submit">Ajouter</button>
          <button class="modal-cancel mform-cancel">Annuler</button>
        </div>
      </div>
    `;

    this.shadowRoot.querySelector('.modal-overlay').addEventListener('click', e => {
      if (e.target === e.currentTarget) this._closeModal();
    });
    this.shadowRoot.querySelector('.modal-cancel').addEventListener('click', () => this._closeModal());

    // Create form
    this.shadowRoot.querySelector('.add-btn').addEventListener('click', () => this._openCreateForm());
    const co = this.shadowRoot.querySelector('.create-overlay');
    co.addEventListener('click', e => { if (e.target === co) this._closeCreateForm(); });
    this.shadowRoot.querySelector('.create-cancel').addEventListener('click', () => this._closeCreateForm());
    co.querySelectorAll('.chip[data-cat]').forEach(c => c.addEventListener('click', () => {
      co.querySelectorAll('.chip[data-cat]').forEach(x => x.classList.remove('active'));
      c.classList.add('active');
    }));
    co.querySelectorAll('.chip[data-repeat]').forEach(c => c.addEventListener('click', () => {
      co.querySelectorAll('.chip[data-repeat]').forEach(x => x.classList.remove('active'));
      c.classList.add('active');
      const r = c.dataset.repeat;
      co.querySelector('.wd-row').classList.toggle('hidden-field', r !== 'semaine');
      co.querySelector('.monthday-row').classList.toggle('hidden-field', r !== 'mois');
      co.querySelector('.date-row').classList.toggle('hidden-field', r !== 'once');
    }));
    co.querySelectorAll('.wd-btn').forEach(b => b.addEventListener('click', () => b.classList.toggle('active')));
    co.querySelector('.critical-chip').addEventListener('click', () => co.querySelector('.critical-chip').classList.toggle('active'));
    this.shadowRoot.querySelector('.create-submit').addEventListener('click', () => this._submitCreateTask());

    // Context menu
    const cx = this.shadowRoot.querySelector('.ctx-overlay');
    cx.addEventListener('click', e => { if (e.target === cx) this._closeCtx(); });
    this.shadowRoot.querySelector('.ctx-cancel').addEventListener('click', () => this._closeCtx());
    this.shadowRoot.querySelector('.ctx-postpone').addEventListener('click', () => this._ctxPostpone());
    this.shadowRoot.querySelector('.ctx-skip').addEventListener('click', () => this._ctxSkip());
    this.shadowRoot.querySelector('.ctx-edit').addEventListener('click', () => this._ctxEdit());
    this.shadowRoot.querySelector('.ctx-delete').addEventListener('click', () => this._ctxDelete());

    // Members
    this.shadowRoot.querySelector('.members-btn').addEventListener('click', () => this._openMembers());
    const mo = this.shadowRoot.querySelector('.members-overlay');
    mo.addEventListener('click', e => { if (e.target === mo) this._closeMembers(); });
    this.shadowRoot.querySelector('.members-close').addEventListener('click', () => this._closeMembers());
    this.shadowRoot.querySelector('.members-add-btn').addEventListener('click', () => this._openMemberForm());

    // Member form
    const mf = this.shadowRoot.querySelector('.mform-overlay');
    mf.addEventListener('click', e => { if (e.target === mf) this._closeMemberForm(); });
    this.shadowRoot.querySelector('.mform-cancel').addEventListener('click', () => this._closeMemberForm());
    this.shadowRoot.querySelector('.mform-submit').addEventListener('click', () => this._submitMemberForm());
    mf.querySelectorAll('.tone-swatch').forEach(s => s.addEventListener('click', () => {
      mf.querySelectorAll('.tone-swatch').forEach(x => x.classList.remove('active'));
      s.classList.add('active');
    }));
    mf.querySelector('.mform-name').addEventListener('input', e => {
      const initial = mf.querySelector('.mform-initial');
      if (!initial._userEdited) initial.value = e.target.value.trim().slice(0, 2).toUpperCase();
    });
    mf.querySelector('.mform-initial').addEventListener('input', e => {
      e.target._userEdited = e.target.value.length > 0;
    });

    this._rendered = true;
    this._seq = this._hass?.states[this._config.foyer_sensor]?.state;
    this._today = foyerToday();
    this._renderTasks();

    // The sensor's state only changes on task/member mutations, so a card left
    // open across midnight with no activity would keep showing yesterday's
    // "today/late" bucketing. Poll for the date rollover and force a re-render.
    this._startDayTimer();
    this._startCriticalTimer();
  }

  // ── Task list render ───────────────────────────────────────────────────────
  _renderTasks() {
    const listEl = this.shadowRoot.querySelector('.task-list');
    if (!listEl) return;

    const data = this._getData();
    if (!data) {
      listEl.innerHTML = '<div class="offline-state">⚠️ Connexion au serveur FOYER indisponible.</div>';
      this._setHeader(0, 0, 0);
      return;
    }

    const { tasks, members, history } = data;
    const today = foyerToday();

    // ── Filter bar ─────────────────────────────────────────────────────────
    if (!this._filter) this._filter = 'all';
    const filterEl = this.shadowRoot.querySelector('.filter-row');
    if (filterEl && members.length > 0) {
      filterEl.innerHTML = [
        { id: 'all', label: 'Tout', tone: null, initial: null },
        ...members,
      ].map(m => {
        const isAll    = m.id === 'all';
        const active   = this._filter === m.id ? ' active' : '';
        const color    = FoyerTasksCard._TONE_CSS[m.tone] ?? '#888';
        const avStyle  = isAll ? '' : `style="background:${color}"`;
        const avInner  = isAll ? '' : `<span class="f-av" ${avStyle}>${this._esc(m.initial)}</span>`;
        return `<button class="filter-chip${active}" data-fid="${this._esc(m.id)}">${avInner}${this._esc(m.id === 'all' ? 'Tout' : m.name)}</button>`;
      }).join('');
      filterEl.querySelectorAll('.filter-chip').forEach(btn =>
        btn.addEventListener('click', () => {
          this._filter = btn.dataset.fid;
          this._renderTasks();
        })
      );
    } else if (filterEl) {
      filterEl.innerHTML = '';
    }

    const filterFn = t => this._filter === 'all' || t.assignee === this._filter || t.assignee === null;

    // JS getUTCDay (0=Sun) → app weekday (0=Mon)
    const appWeekday = iso => (new Date(iso + 'T00:00:00Z').getUTCDay() + 6) % 7;
    // Returns false if a recurring task's due date doesn't match its own recurrence rule
    const ruleMatchesDue = t => {
      if (!t.recurring || !t.weekDays?.length) return true;
      if (t.repeat === 'semaine') return t.weekDays.includes(appWeekday(t.due.slice(0, 10)));
      return true;
    };

    // Historical completion records (done: true, recurring: false) are logs, not active tasks
    const active    = tasks.filter(t => !t.done);
    const todayDone = tasks.filter(t => t.done && !t.recurring && t.due?.slice(0, 10) === today);
    const byDate    = (a, b) => a.due < b.due ? -1 : a.due > b.due ? 1 : 0;
    // Exclude tasks whose due date doesn't match their recurrence rule (server-side data bug)
    const late      = active.filter(t => filterFn(t) && ruleMatchesDue(t) && (t.late || t.due.slice(0, 10) < today)).sort(byDate);
    const todayT    = active.filter(t => filterFn(t) && ruleMatchesDue(t) && !t.late && t.due.slice(0, 10) === today).sort(byDate);
    const future    = active.filter(t => filterFn(t) && (!ruleMatchesDue(t) || (!t.late && t.due.slice(0, 10) > today))).sort(byDate);
    const upcoming  = future.filter(t => t.recurring);
    const planned   = future.filter(t => !t.recurring);

    let html = '';

    const vac = data.vacation;
    const today10 = today;
    if (vac?.active) {
      const vacUntil = vac.until ? new Date(vac.until + 'T12:00:00').toLocaleDateString('fr-FR', { day: 'numeric', month: 'long' }) : '';
      html += `<div style="background:#fef3c7;border-left:3px solid #f59e0b;padding:8px 12px;margin-bottom:8px;border-radius:4px;font-size:12px;font-weight:600;color:#92400e">🏖 Mode vacances actif${vacUntil ? ' jusqu\'au ' + vacUntil : ''}</div>`;
    }

    if (late.length) {
      html += `<div class="section-label">En retard</div>`;
      late.forEach(t => { html += this._rowHTML(t, members); });
    }
    if (todayT.length) {
      html += `<div class="section-label">Aujourd'hui</div>`;
      todayT.forEach(t => { html += this._rowHTML(t, members); });
    }
    const allUpcoming = [...upcoming, ...(this._config.show_upcoming ? planned : [])];
    if (allUpcoming.length) {
      html += `<div class="section-label">À venir</div><div class="upcoming-list">`;
      allUpcoming.forEach(t => { html += this._upcomingRowHTML(t); });
      html += `</div>`;
    }
    if (this._config.show_done && todayDone.length) {
      html += `<div class="section-label">Terminées</div>`;
      todayDone.forEach(t => { html += this._rowHTML(t, members); });
    }
    if (!late.length && !todayT.length && !upcoming.length) {
      html += `
        <div class="empty-state">
          <span class="empty-icon">🎉</span>
          Toutes les tâches sont à jour !
        </div>
      `;
    }

    listEl.innerHTML = html;

    listEl.querySelectorAll('.task-row').forEach(row => {
      const task = tasks.find(t => t.id === row.dataset.task);
      if (!task || task.done) return;

      let startX = 0, startY = 0, pressTimer = null, didLongPress = false, moved = false;

      const onTouchStart = e => {
        startX = e.touches[0].clientX; startY = e.touches[0].clientY;
        didLongPress = false; moved = false;
        pressTimer = setTimeout(() => {
          if (!moved) { didLongPress = true; row.classList.add('pressing'); this._openCtx(task); }
        }, 480);
      };
      const onTouchMove = e => {
        const dx = e.touches[0].clientX - startX;
        const dy = e.touches[0].clientY - startY;
        if (!moved && (Math.abs(dx) > 12 || Math.abs(dy) > 12)) {
          moved = true; clearTimeout(pressTimer); row.classList.remove('pressing');
        }
      };
      const onTouchEnd = () => { clearTimeout(pressTimer); row.classList.remove('pressing'); };

      row.addEventListener('touchstart', onTouchStart, { passive: true });
      row.addEventListener('touchmove',  onTouchMove,  { passive: true });
      row.addEventListener('touchend',   onTouchEnd);
      // Desktop long press on the row itself
      row.addEventListener('mousedown',  () => { didLongPress = false; pressTimer = setTimeout(() => { didLongPress = true; row.classList.add('pressing'); this._openCtx(task); }, 480); });
      row.addEventListener('mouseup',    () => { clearTimeout(pressTimer); row.classList.remove('pressing'); });
      row.addEventListener('mouseleave', () => { clearTimeout(pressTimer); row.classList.remove('pressing'); });
      row.addEventListener('click', () => {
        if (didLongPress) { didLongPress = false; return; }
        this._openModal(task, members);
      });
    });

    // Progress = pending urgent + completions today (from history, covers recurring)
    const todayHistDone = (history ?? []).filter(h => foyerLocalDateOf(h.at) === today);
    const urgentTotal = late.length + todayT.length + todayHistDone.length;
    const urgentDone  = todayHistDone.length;
    this._setHeader(urgentDone, urgentTotal, late.length);
    this._updateCriticalAlerts();
  }

  _setHeader(done, total, lateCount) {
    const pct      = total > 0 ? Math.round((done / total) * 100) : 0;
    const fillEl   = this.shadowRoot.querySelector('.progress-fill');
    const labelEl  = this.shadowRoot.querySelector('.progress-label');
    const pillEl   = this.shadowRoot.querySelector('.late-pill');

    if (fillEl) {
      fillEl.style.width = `${pct}%`;
      fillEl.classList.toggle('done', pct === 100);
    }
    if (labelEl) labelEl.textContent = total > 0 ? `${done} / ${total} faites` : 'Aucune tâche';
    if (pillEl) {
      if (lateCount > 0) {
        pillEl.textContent = `${lateCount} en retard`;
        pillEl.classList.remove('hidden');
      } else {
        pillEl.classList.add('hidden');
      }
    }
  }

  // ── Row HTML ───────────────────────────────────────────────────────────────
  _rowHTML(task, members, isUpcoming = false) {
    const assignee  = task.done
      ? (members.find(m => m.id === task.doneBy) ?? members.find(m => m.id === task.assignee))
      : members.find(m => m.id === task.assignee);
    const icon      = FOYER_CAT_ICONS[task.cat]  ?? '📋';
    const catColor  = FOYER_CAT_COLORS[task.cat] ?? '#888';
    const lateBadge = task.late ? '<span class="late-badge">Retard</span>' : '';
    const dueTime   = task.time ? ` · ${task.time}` : '';
    const isDone    = task.done;

    let recurLabel = '';
    if (task.recurring && task.freqText) {
      recurLabel = task.freqText;
    } else if (!task.recurring && (!task.repeat || task.repeat === 'once')) {
      recurLabel = 'Ponctuelle';
    }

    let metaLine;
    if (isUpcoming) {
      metaLine = `📅 ${this._esc(this._formatDue(task.due))}${dueTime}`;
    } else if (assignee) {
      const color = FoyerTasksCard._TONE_CSS[assignee.tone] ?? '#888';
      metaLine = `<span style="display:inline-flex;align-items:center;gap:5px"><span style="display:inline-flex;align-items:center;justify-content:center;width:16px;height:16px;border-radius:50%;background:${color};color:#fff;font-size:9px;font-weight:700;flex-shrink:0">${this._esc(assignee.initial)}</span>${this._esc(assignee.name)}</span>${dueTime}`;
    } else {
      metaLine = `À prendre${dueTime}`;
    }

    const cl = task.checklist ?? [];
    const clDone = cl.filter(x => x.done).length;
    const clBadge = cl.length
      ? `<span style="font-size:10px;padding:1px 6px;border-radius:999px;background:${clDone===cl.length?'#d1fae5':clDone>0?'#dbeafe':'#f3f4f6'};color:${clDone===cl.length?'#065f46':clDone>0?'#1d4ed8':'#6b7280'};font-weight:600;border:1px solid ${clDone===cl.length?'#a7f3d0':clDone>0?'#bfdbfe':'#e5e7eb'}">📋 ${clDone}/${cl.length}</span>`
      : '';

    const rowClass = isDone ? ' done' : isUpcoming ? ' upcoming' : '';
    const criticalAttr = (task.critical && !isDone) ? ' data-critical="1"' : '';
    const inner = `
      <div class="task-row${rowClass}" data-task="${this._esc(task.id)}"${criticalAttr}>
        <div class="cat-bar" style="background:${catColor}"></div>
        ${task.critical && !isDone ? '<span class="critical-dot"></span>' : ''}
        <span class="task-icon">${icon}</span>
        <div class="task-body">
          <span class="${isDone ? 'task-name striked' : 'task-name'}">${this._esc(task.title)}${lateBadge}${clBadge ? ' ' + clBadge : ''}</span>
          <span class="task-meta">${metaLine}</span>
          ${task.note && !isUpcoming ? `<span class="task-note">${this._esc(task.note)}</span>` : ''}
          ${recurLabel && !isUpcoming ? `<span class="task-recur">${this._esc(recurLabel)}</span>` : ''}
          ${isUpcoming && recurLabel ? `<span class="due-chip">${this._esc(recurLabel)}</span>` : ''}
        </div>
        <span class="task-chevron">${isDone ? '✓' : '›'}</span>
      </div>
    `;
    return inner;
  }

  // ── Upcoming compact row ──────────────────────────────────────────────────
  _upcomingRowHTML(task) {
    const icon = FOYER_CAT_ICONS[task.cat] ?? '📋';
    const due  = this._formatDue(task.due);
    const time = task.time ? ` · ${task.time}` : '';
    return `
      <div class="upcoming-row">
        <span class="upcoming-icon">${icon}</span>
        <span class="upcoming-name">${this._esc(task.title)}</span>
        <span class="upcoming-due">${this._esc(due)}${time}</span>
      </div>
    `;
  }

  // ── Create form ───────────────────────────────────────────────────────────
  _openCreateForm(task = null) {
    this._editingTask = task;
    const co     = this.shadowRoot.querySelector('.create-overlay');
    const repeat = task?.repeat ?? 'semaine';
    co.querySelector('.create-title-input').value = task?.title ?? '';
    co.querySelectorAll('.chip[data-cat]').forEach(c => c.classList.toggle('active', c.dataset.cat === (task?.cat ?? 'maison')));
    co.querySelectorAll('.chip[data-repeat]').forEach(c => c.classList.toggle('active', c.dataset.repeat === repeat));
    co.querySelectorAll('.wd-btn').forEach(b => b.classList.toggle('active', (task?.weekDays ?? []).includes(Number(b.dataset.wd))));
    co.querySelector('.wd-row').classList.toggle('hidden-field', repeat !== 'semaine');
    co.querySelector('.monthday-row').classList.toggle('hidden-field', repeat !== 'mois');
    co.querySelector('.date-row').classList.toggle('hidden-field', repeat !== 'once');
    co.querySelector('.monthday-input').value = task?.monthDay ?? 1;
    co.querySelector('.create-date-input').value = task?.due?.slice(0, 10) || foyerToday();
    co.querySelector('.create-time-input').value = task?.time ?? '18:00';
    co.querySelector('.create-note-input').value  = task?.note ?? '';
    co.querySelector('.critical-chip').classList.toggle('active', !!task?.critical);
    // Populate assignee chips dynamically
    const members = this._getData()?.members ?? [];
    const curAssignee = task?.assignee ?? null;
    const ac = co.querySelector('.assignee-chips');
    ac.innerHTML = [{ id: '__none__', initial: '?', name: 'À prendre', tone: null }, ...members].map(m => `
      <button class="assignee-chip${(m.id === '__none__' ? !curAssignee : m.id === curAssignee) ? ' active' : ''}" data-assignee="${this._esc(m.id)}">
        <div class="assignee-avatar-sm" style="background:${FoyerTasksCard._TONE_CSS[m.tone] ?? '#aaa'}">${this._esc(m.initial)}</div>
        ${this._esc(m.name)}
      </button>`).join('');
    ac.querySelectorAll('.assignee-chip').forEach(c => c.addEventListener('click', () => {
      ac.querySelectorAll('.assignee-chip').forEach(x => x.classList.remove('active'));
      c.classList.add('active');
    }));
    const btn = co.querySelector('.create-submit');
    btn.disabled = false;
    btn.textContent = task ? 'Enregistrer les modifications' : 'Ajouter la tâche';
    co.classList.add('open');
    setTimeout(() => co.querySelector('.create-title-input').focus(), 220);
  }

  _closeCreateForm() {
    this.shadowRoot.querySelector('.create-overlay').classList.remove('open');
    this._editingTask = null;
  }

  // ── Context menu ──────────────────────────────────────────────────────────
  _openCtx(task) {
    this._ctxTask = task;
    const cx = this.shadowRoot.querySelector('.ctx-overlay');
    cx.querySelector('.ctx-task-name').textContent = task.title;
    // "Ignorer" only makes sense for recurring tasks
    cx.querySelector('.ctx-skip').style.display = task.recurring ? '' : 'none';
    cx.classList.add('open');
  }

  _closeCtx() {
    this.shadowRoot.querySelector('.ctx-overlay').classList.remove('open');
    this._ctxTask = null;
  }

  _ctxPostpone() {
    const task = this._ctxTask;
    this._closeCtx();
    if (!task) return;
    const members = this._getData()?.members ?? [];
    this._openModal(task, members, 'postpone');
  }

  _ctxSkip() {
    const task = this._ctxTask;
    this._closeCtx();
    if (!task) return;
    const members = this._getData()?.members ?? [];
    this._openModal(task, members, 'skip');
  }

  async _confirmPostpone(memberId) {
    if (!this._pendingTask || !this._hass) return;
    const task = this._pendingTask;
    this._closeModal();
    const tomorrow = this._addDaysISO(foyerToday(), 1);
    const time = task.time || task.due.slice(11, 16) || '18:00';
    await this._callApi({
      type: 'postponeTask', id: task.id, memberId,
      at: new Date().toISOString(), histId: this._newId(),
      patch: { due: `${tomorrow}T${time}`, late: false },
    });
  }

  async _confirmSkip(memberId) {
    if (!this._pendingTask || !this._hass) return;
    const task = this._pendingTask;
    this._closeModal();
    const after = task.due.slice(0, 10);
    const next  = this._nextOccurrenceAfter(task, after);
    const time  = task.time || task.due.slice(11, 16) || '18:00';
    const op = { type: 'skipTask', id: task.id, memberId, at: new Date().toISOString(), histId: this._newId() };
    if (next) op.patch = { due: `${next}T${time}`, late: false };
    await this._callApi(op);
  }

  _ctxEdit() {
    const task = this._ctxTask;
    this._closeCtx();
    if (!task) return;
    this._openCreateForm(task);
  }

  async _ctxDelete() {
    const task = this._ctxTask;
    this._closeCtx();
    if (!task) return;
    await this._callApi({ type: 'deleteTask', id: task.id });
  }

  // ── Members management ───────────────────────────────────────────────────
  static get _TONE_CSS() {
    return { rose:'#ec4899', peach:'#fb923c', amber:'#f59e0b', mint:'#10b981', sky:'#0ea5e9', lilac:'#a78bfa' };
  }

  _openMembers() {
    const members = this._getData()?.members ?? [];
    const list = this.shadowRoot.querySelector('.members-list');
    list.innerHTML = members.length
      ? members.map(m => `
          <div class="member-row" data-member="${this._esc(m.id)}">
            <div class="member-avatar" style="background:${FoyerTasksCard._TONE_CSS[m.tone] ?? '#888'}">${this._esc(m.initial)}</div>
            <div class="member-info"><div class="member-name">${this._esc(m.name)}</div></div>
            <button class="member-delete" data-member="${this._esc(m.id)}" title="Supprimer">✕</button>
          </div>
        `).join('')
      : '<div style="padding:16px 4px;color:var(--secondary-text-color);font-size:14px">Aucun membre pour l\'instant.</div>';

    list.querySelectorAll('.member-row').forEach(row => {
      row.addEventListener('click', e => {
        if (e.target.closest('.member-delete')) return;
        const m = (this._getData()?.members ?? []).find(x => x.id === row.dataset.member);
        if (m) this._openMemberForm(m);
      });
    });
    list.querySelectorAll('.member-delete').forEach(btn => {
      btn.addEventListener('click', async e => {
        e.stopPropagation();
        const id = btn.dataset.member;
        await this._callApi({ type: 'deleteMember', id });
        // Refresh list after deletion
        setTimeout(() => this._openMembers(), 300);
      });
    });

    this.shadowRoot.querySelector('.members-overlay').classList.add('open');
  }

  _closeMembers() {
    this.shadowRoot.querySelector('.members-overlay').classList.remove('open');
  }

  _openMemberForm(member = null) {
    this._editingMember = member;
    const mf = this.shadowRoot.querySelector('.mform-overlay');
    mf.querySelector('.mform-title').textContent = member ? 'Modifier le membre' : 'Nouveau membre';
    mf.querySelector('.mform-name').value = member?.name ?? '';
    const initInput = mf.querySelector('.mform-initial');
    initInput.value = member?.initial ?? '';
    initInput._userEdited = !!member;
    const tone = member?.tone ?? 'sky';
    mf.querySelectorAll('.tone-swatch').forEach(s => s.classList.toggle('active', s.dataset.tone === tone));
    mf.querySelector('.mform-submit').textContent = member ? 'Enregistrer' : 'Ajouter';
    mf.classList.add('open');
    setTimeout(() => mf.querySelector('.mform-name').focus(), 220);
  }

  _closeMemberForm() {
    this.shadowRoot.querySelector('.mform-overlay').classList.remove('open');
    this._editingMember = null;
  }

  async _submitMemberForm() {
    const mf   = this.shadowRoot.querySelector('.mform-overlay');
    const btn  = mf.querySelector('.mform-submit');
    const name = mf.querySelector('.mform-name').value.trim();
    if (!name) { mf.querySelector('.mform-name').style.borderColor = 'var(--error-color, #f44336)'; return; }
    const initial = (mf.querySelector('.mform-initial').value.trim() || name.slice(0, 2)).toUpperCase();
    const tone = mf.querySelector('.tone-swatch.active')?.dataset.tone ?? 'sky';
    btn.disabled = true; btn.textContent = '…';
    const op = this._editingMember
      ? { type: 'editMember', id: this._editingMember.id, patch: { name, initial, tone } }
      : { type: 'addMember', id: this._newId(), member: { name, initial, tone } };
    await this._callApi(op);
    this._closeMemberForm();
    setTimeout(() => this._openMembers(), 300);
  }

  get _apiBase() {
    return this._config?.api_base ?? `http://${location.hostname}:8787`;
  }

  async _callApi(op) {
    const resp = await fetch(this._apiBase + '/api/ops', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(op),
    });
    if (!resp.ok) console.error('[foyer] API error', resp.status, op);
  }

  _nextOccurrenceAfter(task, afterISO) {
    const appWd = iso => (new Date(iso + 'T00:00:00Z').getUTCDay() + 6) % 7;
    let d = this._addDaysISO(afterISO, 1);
    for (let i = 0; i < 366; i++) {
      if (task.repeat === 'jour') return d;
      if (task.repeat === 'semaine' && task.weekDays?.length && task.weekDays.includes(appWd(d))) return d;
      if (task.repeat === 'mois') {
        const day = new Date(d + 'T00:00:00Z').getUTCDate();
        if (day === (task.monthDay ?? 1)) return d;
      }
      d = this._addDaysISO(d, 1);
    }
    return null;
  }

  async _submitCreateTask() {
    const co  = this.shadowRoot.querySelector('.create-overlay');
    const btn = co.querySelector('.create-submit');
    const title = co.querySelector('.create-title-input').value.trim();
    if (!title) { co.querySelector('.create-title-input').focus(); return; }

    const cat    = co.querySelector('.chip[data-cat].active')?.dataset.cat ?? 'maison';
    const repeat = co.querySelector('.chip[data-repeat].active')?.dataset.repeat ?? 'semaine';
    const time   = co.querySelector('.create-time-input').value || '18:00';

    const weekDays = repeat === 'semaine'
      ? [...co.querySelectorAll('.wd-btn.active')].map(b => Number(b.dataset.wd))
      : [];
    if (repeat === 'semaine' && !weekDays.length) {
      co.querySelector('.wd-row').style.outline = '2px solid var(--error-color, #f44336)';
      co.querySelector('.wd-row').style.borderRadius = '8px';
      setTimeout(() => { co.querySelector('.wd-row').style.outline = ''; }, 1200);
      return;
    }

    const monthDay = repeat === 'mois' ? (Number(co.querySelector('.monthday-input').value) || 1) : 1;
    const date     = repeat === 'once' ? co.querySelector('.create-date-input').value : '';
    const due      = this._firstDue(repeat, weekDays, monthDay, time, date);
    const freqText = this._freqText(repeat, weekDays, monthDay, time);
    const id       = this._newId();

    btn.disabled = true;
    btn.textContent = '…';

    const assigneeVal = co.querySelector('.assignee-chip.active')?.dataset.assignee;
    const assignee    = (!assigneeVal || assigneeVal === '__none__') ? null : assigneeVal;
    const note        = co.querySelector('.create-note-input').value.trim() || null;
    const critical    = co.querySelector('.critical-chip').classList.contains('active');
    const patch = { title, cat, repeat, recurring: repeat !== 'once', weekDays, monthDay, time, freqText, due, late: false, assignee, note, critical };
    const op = this._editingTask
      ? { type: 'editTask', id: this._editingTask.id, patch }
      : { type: 'addTask', id: this._newId(), task: { ...patch, done: false, doneBy: null, doneAt: null } };

    try {
      const resp = await fetch(this._apiBase + '/api/ops', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(op),
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      this._closeCreateForm();
    } catch(e) {
      console.error('[foyer] submitTask', e);
      btn.textContent = 'Erreur — réessayer';
      btn.disabled = false;
    }
  }

  _newId() {
    return 'ta' + Math.random().toString(36).slice(2, 9) + Math.random().toString(36).slice(2, 9);
  }

  _addDaysISO(iso, n) {
    const d = new Date(iso + 'T00:00:00Z');
    d.setUTCDate(d.getUTCDate() + n);
    return d.toISOString().slice(0, 10);
  }

  _firstDue(repeat, weekDays, monthDay, time, date = '') {
    const today  = foyerToday();
    const appWd  = iso => (new Date(iso + 'T00:00:00Z').getUTCDay() + 6) % 7;
    if (repeat === 'once') return `${date || today}T${time}`;
    if (repeat === 'jour') return `${today}T${time}`;
    if (repeat === 'semaine') {
      let d = today;
      for (let i = 0; i < 7; i++) {
        if (weekDays.includes(appWd(d))) return `${d}T${time}`;
        d = this._addDaysISO(d, 1);
      }
      return `${today}T${time}`;
    }
    if (repeat === 'mois') {
      const now = new Date();
      let y = now.getFullYear(), m = now.getMonth() + 1;
      if (now.getDate() > monthDay) { m++; if (m > 12) { m = 1; y++; } }
      const maxD = new Date(Date.UTC(y, m, 0)).getUTCDate();
      return `${y}-${String(m).padStart(2,'0')}-${String(Math.min(monthDay, maxD)).padStart(2,'0')}T${time}`;
    }
    return `${today}T${time}`;
  }

  _freqText(repeat, weekDays, monthDay, time) {
    const D = ['lundis','mardis','mercredis','jeudis','vendredis','samedis','dimanches'];
    if (repeat === 'once')   return 'Ponctuelle';
    if (repeat === 'jour')   return `Tous les jours à ${time}`;
    if (repeat === 'semaine') {
      if (weekDays.length === 1) return `Tous les ${D[weekDays[0]]} à ${time}`;
      return `Tous les ${[...weekDays].sort().map(d => D[d].slice(0,3)).join(', ')} à ${time}`;
    }
    if (repeat === 'mois')  return `Le ${monthDay} de chaque mois à ${time}`;
    return '';
  }

  // ── Modal ──────────────────────────────────────────────────────────────────
  static get _MODAL_EYEBROW() {
    return {
      complete: 'Qui a fait ça ?',
      postpone: 'Qui décale la tâche ?',
      skip:     'Qui ignore cette tâche ?',
    };
  }

  _openModal(task, members, action = 'complete') {
    this._pendingTask   = task;
    this._pendingAction = action;
    this.shadowRoot.querySelector('.modal-eyebrow').textContent = FoyerTasksCard._MODAL_EYEBROW[action] ?? FoyerTasksCard._MODAL_EYEBROW.complete;
    this.shadowRoot.querySelector('.modal-task-name').textContent = task.title;

    const peopleRow = this.shadowRoot.querySelector('.people-row');
    peopleRow.innerHTML = members.map(m => `
      <button class="person-btn" data-member-id="${this._esc(m.id)}"
              style="--c: ${FOYER_TONE_COLORS[m.tone] ?? '#888'}">
        <div class="person-avatar">${this._esc(m.initial)}</div>
        <span class="person-name">${this._esc(m.name)}</span>
      </button>
    `).join('');

    peopleRow.querySelectorAll('.person-btn').forEach(btn => {
      btn.addEventListener('click', e => {
        e.stopPropagation();
        const memberId = btn.dataset.memberId;
        if (this._pendingAction === 'postpone') this._confirmPostpone(memberId);
        else if (this._pendingAction === 'skip') this._confirmSkip(memberId);
        else this._confirmComplete(memberId);
      });
    });

    this.shadowRoot.querySelector('.modal-overlay').classList.add('open');
  }

  _closeModal() {
    this.shadowRoot.querySelector('.modal-overlay')?.classList.remove('open');
    this._pendingTask   = null;
    this._pendingAction = null;
  }

  _confirmComplete(memberId) {
    if (!this._pendingTask || !this._hass) return;
    const task = this._pendingTask;
    this._closeModal();
    this._callApi({
      type: 'completeTask', id: task.id, memberId,
      at: new Date().toISOString(),
      newId: this._newId(), histId: this._newId(),
    });
    const row = this.shadowRoot.querySelector(`[data-task="${task.id}"]`);
    if (row) row.style.display = 'none';
  }

  // ── Utilities ──────────────────────────────────────────────────────────────
  _formatDue(dueIso) {
    const date = new Date(dueIso);
    const now  = new Date();
    const diffDays = Math.round((date - now) / (1000 * 60 * 60 * 24));
    if (diffDays < 2)  return 'Demain';
    if (diffDays < 7)  return date.toLocaleDateString('fr-FR', { weekday: 'long' });
    return date.toLocaleDateString('fr-FR', { weekday: 'short', day: 'numeric', month: 'short' });
  }

  _formatAt(isoString) {
    const date = foyerParseAt(isoString);
    const now  = new Date();
    const opts = { day: 'numeric', month: 'long' };
    const todayStr     = now.toLocaleDateString('fr-FR', opts);
    const yesterdayStr = new Date(now - 864e5).toLocaleDateString('fr-FR', opts);
    const dateStr      = date.toLocaleDateString('fr-FR', opts);
    const timeStr      = date.toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
    if (dateStr === todayStr)     return `Aujourd'hui à ${timeStr}`;
    if (dateStr === yesterdayStr) return `Hier à ${timeStr}`;
    return `${dateStr} à ${timeStr}`;
  }

  _esc(str) {
    return String(str ?? '')
      .replace(/&/g, '&amp;').replace(/</g, '&lt;')
      .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  getCardSize() { return 6; }
}

customElements.define('foyer-tasks-card', FoyerTasksCard);

// ─────────────────────────────────────────────────────────────────────────────
//  FoyerHistoryCard — liste compacte des tâches récemment complétées
// ─────────────────────────────────────────────────────────────────────────────
class FoyerHistoryCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this._hass   = null;
    this._config = null;
    this._seq    = null;
  }

  setConfig(config) {
    if (!config.foyer_sensor) throw new Error('foyer-history-card : "foyer_sensor" est requis.');
    this._config = { title: 'Historique', history_count: 20, ...config };
    if (this._hass) this._render();
  }

  set hass(hass) {
    const seq = hass.states[this._config?.foyer_sensor]?.state;
    this._hass = hass;
    if (this._config && seq !== this._seq) {
      this._seq = seq;
      this._rendered ? this._renderHistory() : this._render();
    }
  }

  _getData() {
    const state = this._hass?.states[this._config?.foyer_sensor];
    if (!state) return null;
    return {
      history: state.attributes.history ?? [],
      members: state.attributes.members ?? [],
    };
  }

  _render() {
    this._rendered = true;
    this.shadowRoot.innerHTML = `
      <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        :host { display: block; }
        .card {
          background: var(--card-background-color);
          border-radius: var(--ha-card-border-radius, 12px);
          box-shadow: var(--ha-card-box-shadow, 0 2px 8px rgba(0,0,0,.1));
          overflow: hidden;
          padding-bottom: 8px;
        }
        .card-header {
          padding: 16px 20px 10px;
          font-size: 15px;
          font-weight: 600;
          color: var(--primary-text-color);
        }
        .hist-list { padding: 0 16px 4px; }
        .hist-row {
          display: flex;
          align-items: center;
          gap: 8px;
          padding: 7px 0;
          border-bottom: 1px solid var(--divider-color, rgba(0,0,0,.06));
        }
        .hist-row:last-child { border-bottom: none; }
        .hist-dot {
          width: 8px; height: 8px;
          border-radius: 50%;
          flex-shrink: 0;
        }
        .hist-icon { font-size: 14px; flex-shrink: 0; width: 18px; text-align: center; }
        .hist-name {
          flex: 1;
          font-size: 13px;
          font-weight: 500;
          color: var(--primary-text-color);
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .hist-meta {
          font-size: 11px;
          color: var(--secondary-text-color);
          white-space: nowrap;
          flex-shrink: 0;
        }
        .empty-state {
          padding: 20px;
          text-align: center;
          color: var(--secondary-text-color);
          font-size: 13px;
        }
      </style>
      <div class="card">
        <div class="card-header">${foyerEsc(this._config.title)}</div>
        <div class="hist-list"></div>
      </div>
    `;
    this._renderHistory();
  }

  _renderHistory() {
    const listEl = this.shadowRoot.querySelector('.hist-list');
    if (!listEl) return;
    const data = this._getData();
    if (!data) { listEl.innerHTML = '<div class="empty-state">En attente des données…</div>'; return; }

    const { history, members } = data;
    const recent = [...history]
      .sort((a, b) => b.at.localeCompare(a.at))
      .slice(0, this._config.history_count);

    if (!recent.length) {
      listEl.innerHTML = '<div class="empty-state">Aucune tâche complétée récemment.</div>';
      return;
    }

    listEl.innerHTML = recent.map(entry => {
      const member   = members.find(m => m.id === entry.by);
      const catColor = FOYER_CAT_COLORS[entry.cat] ?? '#888';
      const catIcon  = FOYER_CAT_ICONS[entry.cat]  ?? '📋';
      const tone     = FoyerTasksCard._TONE_CSS[member?.tone] ?? catColor;
      const avatar   = member
        ? `<span style="display:inline-flex;align-items:center;justify-content:center;width:14px;height:14px;border-radius:50%;background:${tone};color:#fff;font-size:8px;font-weight:700;flex-shrink:0">${foyerEsc(member.initial)}</span>`
        : '';
      const actionMeta = {
        postponed: { icon: '⏩', verb: 'a reporté' },
        skipped:   { icon: '⏭', verb: 'a ignoré' },
      }[entry.action];
      const rowIcon = actionMeta?.icon ?? catIcon;
      const verb    = actionMeta?.verb ?? null;
      return `
        <div class="hist-row">
          <div class="hist-dot" style="background:${catColor}"></div>
          <span class="hist-icon">${rowIcon}</span>
          <span class="hist-name">${foyerEsc(entry.title)}</span>
          <span class="hist-meta" style="display:flex;align-items:center;gap:4px">${avatar}${foyerEsc(member?.name ?? '?')}${verb ? ` ${verb}` : ''} · ${foyerFormatAt(entry.at)}</span>
        </div>
      `;
    }).join('');
  }

  getCardSize() { return 4; }
}

customElements.define('foyer-history-card', FoyerHistoryCard);

// ─────────────────────────────────────────────────────────────────────────────
//  FoyerProgressCard — anneau SVG avec progression tâches faites / total
// ─────────────────────────────────────────────────────────────────────────────
class FoyerProgressCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this._hass     = null;
    this._config   = null;
    this._seq      = null;
    this._today    = null;
    this._dayTimer = null;
  }

  connectedCallback() {
    // HA/Lovelace can detach and reattach this element (view switches, dashboard
    // relayout) without recreating it, which would otherwise leave the day-change
    // poller dead forever since disconnectedCallback tears it down below.
    this._startDayTimer();
  }

  disconnectedCallback() {
    if (this._dayTimer) { clearInterval(this._dayTimer); this._dayTimer = null; }
  }

  _startDayTimer() {
    if (this._dayTimer || !this._hass) return;
    this._dayTimer = setInterval(() => {
      const d = foyerToday();
      if (d !== this._today) { this._today = d; this._render(); }
    }, 60000);
  }

  setConfig(config) {
    if (!config.foyer_sensor) throw new Error('foyer-progress-card : "foyer_sensor" est requis.');
    this._config = { title: 'Progression du foyer', ...config };
    if (this._hass) this._render();
  }

  set hass(hass) {
    const seq = hass.states[this._config?.foyer_sensor]?.state;
    this._hass = hass;
    if (seq !== this._seq) { this._seq = seq; this._render(); }
    this._startDayTimer();
  }

  _render() {
    const state = this._hass?.states[this._config.foyer_sensor];

    if (!state || state.state === 'unavailable' || state.state === 'unknown') {
      this.shadowRoot.innerHTML = `
        <style>:host{display:block;} ha-card{padding:16px;}</style>
        <ha-card><div style="color:var(--warning-color,#ff9800);font-size:13px">⚠ FOYER indisponible</div></ha-card>
      `;
      return;
    }

    const tasks    = state.attributes.tasks   ?? [];
    const history  = state.attributes.history ?? [];
    const today    = foyerToday();
    this._today    = today;
    const appWd    = iso => (new Date(iso + 'T00:00:00Z').getUTCDay() + 6) % 7;
    const ruleOk   = t => !t.recurring || !t.weekDays?.length || t.repeat !== 'semaine' || t.weekDays.includes(appWd(t.due?.slice(0, 10) ?? today));
    // doneToday: all completions today (covers both recurring and non-recurring)
    const active    = tasks.filter(t => !t.done);
    const doneToday = history.filter(h => foyerLocalDateOf(h.at) === today);
    const late      = active.filter(t => ruleOk(t) && (t.late || t.due?.slice(0, 10) < today)).length;
    const todayPend = active.filter(t => ruleOk(t) && !t.late && t.due?.slice(0, 10) === today).length;
    const total     = late + todayPend + doneToday.length;
    const done      = doneToday.length;
    const pending   = total - done;
    const pct       = total > 0 ? Math.round((done / total) * 100) : 0;

    const R    = 38;
    const circ = +(2 * Math.PI * R).toFixed(2);
    const off  = +(circ - (pct / 100) * circ).toFixed(2);

    const ringColor = late > 0
      ? 'var(--error-color, #f44336)'
      : pct === 100
        ? 'var(--success-color, #4caf50)'
        : 'var(--primary-color, #03a9f4)';

    const catBreakdown = Object.entries(FOYER_CAT_COLORS).map(([cat, color]) => {
      const catActive  = active.filter(t => t.cat === cat && ruleOk(t) && (t.late || t.due?.slice(0,10) <= today));
      const catDone    = doneToday.filter(h => h.cat === cat).length;
      const catTasks   = catActive.length + catDone;
      if (!catTasks) return '';
      const catPct = Math.round((catDone / catTasks) * 100);
      return `
        <div class="cat-row">
          <span class="cat-icon">${FOYER_CAT_ICONS[cat]}</span>
          <div class="cat-bar-wrap">
            <div class="cat-bar-fill" style="width:${catPct}%;background:${color}"></div>
          </div>
          <span class="cat-label">${catDone}/${catTasks}</span>
        </div>
      `;
    }).join('');

    const weekStart = (() => { const d = new Date(); d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); return `${d.getFullYear()}-${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')}`; })();
    const weekCount = history.filter(h => foyerLocalDateOf(h.at) >= weekStart).length;

    this.shadowRoot.innerHTML = `
      <style>
        *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
        :host { display: block; }
        ha-card {
          background: var(--card-background-color);
          border-radius: var(--ha-card-border-radius, 12px);
          box-shadow: var(--ha-card-box-shadow, 0 2px 8px rgba(0,0,0,.1));
          padding: 20px 24px;
          display: flex;
          align-items: center;
          gap: 20px;
        }

        /* ── Ring ── */
        .ring-wrap {
          position: relative;
          width: 92px;
          height: 92px;
          flex-shrink: 0;
        }
        .ring-wrap svg { transform: rotate(-90deg); display: block; }
        .ring-bg {
          fill: none;
          stroke: var(--divider-color, rgba(0,0,0,.1));
          stroke-width: 7;
        }
        .ring-fg {
          fill: none;
          stroke: ${ringColor};
          stroke-width: 7;
          stroke-linecap: round;
          stroke-dasharray: ${circ};
          stroke-dashoffset: ${off};
          transition: stroke-dashoffset .5s cubic-bezier(.4,0,.2,1), stroke .3s ease;
        }
        .ring-label {
          position: absolute;
          inset: 0;
          display: flex;
          flex-direction: column;
          align-items: center;
          justify-content: center;
          gap: 1px;
        }
        .ring-pct  { font-size: 22px; font-weight: 800; color: var(--primary-text-color); line-height: 1; }
        .ring-sub  { font-size: 10px; color: var(--secondary-text-color); letter-spacing: .02em; }

        /* ── Info ── */
        .info { flex: 1; min-width: 0; }
        .info-title {
          font-size: 14px;
          font-weight: 700;
          color: var(--primary-text-color);
          margin-bottom: 8px;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }
        .info-stat {
          font-size: 12px;
          color: var(--secondary-text-color);
          margin-bottom: 2px;
        }
        .info-late {
          font-size: 12px;
          color: var(--error-color, #f44336);
          font-weight: 700;
          margin-bottom: 10px;
        }

        .week-stat {
          font-size: 11px;
          color: var(--secondary-text-color);
          margin-top: 12px;
          padding-top: 10px;
          border-top: 1px solid var(--divider-color, rgba(0,0,0,.06));
        }

        /* ── Category bars ── */
        .cat-row {
          display: flex;
          align-items: center;
          gap: 8px;
          margin-top: 6px;
        }
        .cat-icon { font-size: 14px; width: 18px; text-align: center; flex-shrink: 0; }
        .cat-bar-wrap {
          flex: 1;
          height: 4px;
          background: var(--divider-color, rgba(0,0,0,.1));
          border-radius: 2px;
          overflow: hidden;
        }
        .cat-bar-fill {
          height: 100%;
          border-radius: 2px;
          transition: width .4s ease;
        }
        .cat-label { font-size: 10px; color: var(--secondary-text-color); width: 28px; text-align: right; flex-shrink: 0; }
      </style>

      <ha-card>
        <div class="ring-wrap">
          <svg width="92" height="92" viewBox="0 0 92 92">
            <circle class="ring-bg" cx="46" cy="46" r="${R}"/>
            <circle class="ring-fg" cx="46" cy="46" r="${R}"/>
          </svg>
          <div class="ring-label">
            <span class="ring-pct">${pct}%</span>
            <span class="ring-sub">${done} / ${total}</span>
          </div>
        </div>

        <div class="info">
          <div class="info-title">${this._esc(this._config.title)}</div>
          ${late > 0
            ? `<div class="info-late">⚠ ${late} en retard</div>`
            : `<div class="info-stat">${pending} restante${pending !== 1 ? 's' : ''}</div>`
          }
          ${catBreakdown}
          ${weekCount > 0 ? `<div class="week-stat">🗓 ${weekCount} complétée${weekCount > 1 ? 's' : ''} cette semaine</div>` : ''}
        </div>
      </ha-card>
    `;
  }

  _esc(str) {
    return String(str ?? '')
      .replace(/&/g, '&amp;').replace(/</g, '&lt;')
      .replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  getCardSize() { return 2; }
}

customElements.define('foyer-progress-card', FoyerProgressCard);

console.info(`%c foyer-tasks-card v${FOYER_VERSION} `, 'background:#6366f1;color:#fff;border-radius:3px;padding:2px 6px');

// ── Registration ──────────────────────────────────────────────────────────────
window.customCards = window.customCards || [];
window.customCards.push(
  {
    type:        'foyer-tasks-card',
    name:        'Foyer — Tâches',
    description: 'Tâches du foyer synchronisées en temps réel avec l\'app FOYER (via MQTT)',
    preview:     true,
  },
  {
    type:        'foyer-progress-card',
    name:        'Foyer — Progression',
    description: 'Anneau de progression des tâches avec répartition par catégorie',
    preview:     true,
  }
);
