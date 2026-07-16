// foyer-weekly-card — Lovelace custom card for "libre service" weekly chores
// (tasks due once, any day of the week, resettable Monday-to-Monday).
// Fetches directly from the Taskflow backend API (no HA sensor needed).

const FOYER_WEEKLY_CAT_ICONS  = { cuisine: '🍳', maison: '🏠', linge: '👕', courses: '🛒', animaux: '🐾', sante: '💊', jardinage: '🌿', enfants: '👶' };
const FOYER_WEEKLY_TONE_COLORS = {
  rose: '#ec4899', peach: '#fb923c', amber: '#f59e0b',
  mint: '#10b981', sky: '#0ea5e9', lilac: '#a78bfa',
};

const FOYER_WEEKLY_CSS = `
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :host { display: block; }
  ha-card {
    background: var(--card-background-color);
    border-radius: var(--ha-card-border-radius, 12px);
    overflow: hidden;
    padding-bottom: 8px;
  }
  .card-header {
    padding: 16px 16px 4px;
    font-size: 16px;
    font-weight: 600;
    color: var(--primary-text-color);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .card-sub {
    padding: 0 16px 8px;
    font-size: 12px;
    color: var(--secondary-text-color);
  }
  .task-row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--divider-color, #f0f0f0);
    cursor: pointer;
    transition: background .15s;
  }
  .task-row:last-child { border-bottom: none; }
  .task-row:hover { background: var(--secondary-background-color, #f9fafb); }
  .task-ico { font-size: 18px; flex-shrink: 0; width: 24px; text-align: center; }
  .task-info { flex: 1; min-width: 0; }
  .task-title { font-size: 14px; font-weight: 500; color: var(--primary-text-color); }
  .task-sub { font-size: 11px; color: var(--secondary-text-color); margin-top: 1px; }
  .badge {
    font-size: 11px; font-weight: 600; padding: 2px 8px;
    border-radius: 999px; white-space: nowrap; flex-shrink: 0;
  }
  .badge-late  { background: #fee2e2; color: #b91c1c; }
  .badge-last  { background: #fef3c7; color: #92400e; }
  .badge-soon  { background: #eff6ff; color: #1d4ed8; }
  .badge-done  { background: #d1fae5; color: #065f46; }
  .card-empty {
    padding: 32px 16px;
    text-align: center;
    font-size: 14px;
    color: var(--secondary-text-color);
  }
  .card-empty .ico { font-size: 40px; margin-bottom: 8px; }
  .error { padding: 12px 16px; color: var(--error-color, #dc2626); font-size: 13px; }
  .section-label {
    padding: 10px 16px 4px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: .04em;
    text-transform: uppercase;
    color: var(--secondary-text-color);
  }

  .picker-overlay {
    position: fixed; inset: 0;
    background: rgba(0,0,0,.5);
    display: flex; align-items: flex-end; justify-content: center;
    opacity: 0; pointer-events: none;
    transition: opacity .2s;
    z-index: 10;
  }
  @media (min-width: 480px) { .picker-overlay { align-items: center; } }
  .picker-overlay.open { opacity: 1; pointer-events: auto; }
  .picker-modal {
    background: var(--card-background-color, #fff);
    border-radius: 16px 16px 0 0;
    padding: 20px;
    width: 100%; max-width: 380px;
    transform: translateY(24px);
    transition: transform .2s;
  }
  @media (min-width: 480px) { .picker-modal { border-radius: 16px; transform: scale(.95); } }
  .picker-overlay.open .picker-modal { transform: none; }
  .picker-eyebrow { font-size: 12px; color: var(--secondary-text-color); margin-bottom: 2px; }
  .picker-task-name { font-size: 16px; font-weight: 700; color: var(--primary-text-color); margin-bottom: 16px; }
  .people-row { display: flex; flex-wrap: wrap; gap: 12px; }
  .person-btn {
    display: flex; flex-direction: column; align-items: center; gap: 4px;
    background: none; border: none; cursor: pointer; padding: 4px;
  }
  .person-avatar {
    width: 44px; height: 44px; border-radius: 50%;
    display: flex; align-items: center; justify-content: center;
    background: var(--c, #888); color: #fff; font-weight: 700; font-size: 16px;
  }
  .person-name { font-size: 11px; color: var(--primary-text-color); }
  .picker-cancel {
    margin-top: 16px; width: 100%; padding: 10px;
    border: none; border-radius: 10px;
    background: var(--secondary-background-color, #f0f0f0);
    color: var(--primary-text-color); font-size: 13px; font-weight: 600;
    cursor: pointer;
  }
`;

class FoyerWeeklyCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this._tasks = [];
    this._members = [];
    this._loading = true;
    this._error = null;
    this._pollTimer = null;
    this._pendingTask = null;
  }

  setConfig(config) {
    this._config = config;
    this._startPolling();
  }

  connectedCallback() {
    this._startPolling();
  }

  disconnectedCallback() {
    if (this._pollTimer) clearInterval(this._pollTimer);
  }

  // HA compatibility — hass is passed but we don't need it for API calls
  set hass(hass) {
    this._hass = hass;
  }

  get _apiBase() {
    return this._config?.api_base ?? `http://${location.hostname}:8787`;
  }

  _startPolling() {
    if (this._pollTimer) clearInterval(this._pollTimer);
    this._fetchData();
    this._pollTimer = setInterval(() => this._fetchData(), 30000);
  }

  async _fetchData() {
    try {
      const [tasksR, membersR] = await Promise.all([
        fetch(`${this._apiBase}/api/tasks`),
        fetch(`${this._apiBase}/api/members`),
      ]);
      if (!tasksR.ok) throw new Error(`HTTP ${tasksR.status}`);
      if (!membersR.ok) throw new Error(`HTTP ${membersR.status}`);
      this._tasks = await tasksR.json();
      this._members = await membersR.json();
      this._error = null;
      this._loading = false;
    } catch (e) {
      this._error = e.message;
      this._loading = false;
    }
    this._render();
  }

  _weeklyTasks() {
    return (this._tasks || []).filter(t => t.recurring && t.repeat === 'semaine_libre');
  }

  _daysLeft(due) {
    const ms = new Date(due) - new Date();
    return Math.ceil(ms / 86400000);
  }

  _fdate(iso) {
    if (!iso) return '';
    try {
      return new Date(iso).toLocaleDateString('fr-FR', { day: '2-digit', month: 'short' })
        + ' à ' + new Date(iso).toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' });
    } catch { return iso; }
  }

  _render() {
    const root = this.shadowRoot;
    root.innerHTML = '';

    const style = document.createElement('style');
    style.textContent = FOYER_WEEKLY_CSS;
    root.appendChild(style);

    const card = document.createElement('ha-card');
    const title = this._config?.title || 'Tâches hebdomadaires';
    card.innerHTML = `<div class="card-header">🗓️ ${this._esc(title)}</div>`;

    if (this._loading) {
      card.innerHTML += '<div class="card-empty"><p>Chargement…</p></div>';
      root.appendChild(card);
      return;
    }

    if (this._error) {
      card.innerHTML += `<div class="error">⚠ Impossible de contacter le backend : ${this._esc(this._error)}</div>`;
      root.appendChild(card);
      return;
    }

    const weekly = this._weeklyTasks();
    if (!weekly.length) {
      card.innerHTML += '<div class="card-empty"><div class="ico">🗓️</div><p>Aucune tâche hebdomadaire.<br>Ajoutez-en via l\'interface admin.</p></div>';
      root.appendChild(card);
      return;
    }

    const pending = weekly.filter(t => !t.done).sort((a, b) => a.due < b.due ? -1 : 1);
    const done = weekly.filter(t => t.done).sort((a, b) => a.title.localeCompare(b.title));

    card.innerHTML += `<div class="card-sub">${pending.length} à faire cette semaine · n'importe quel jour</div>`;

    if (pending.length) {
      const list = document.createElement('div');
      pending.forEach(t => list.appendChild(this._taskRow(t)));
      card.appendChild(list);
    } else {
      card.innerHTML += '<div class="card-empty"><div class="ico">🎉</div><p>Toutes les tâches de la semaine sont faites !</p></div>';
    }

    if (done.length) {
      const label = document.createElement('div');
      label.className = 'section-label';
      label.textContent = 'Faites cette semaine';
      card.appendChild(label);
      const list = document.createElement('div');
      done.forEach(t => list.appendChild(this._taskRow(t, true)));
      card.appendChild(list);
    }

    // Member picker modal
    const overlay = document.createElement('div');
    overlay.className = 'picker-overlay';
    overlay.innerHTML = `
      <div class="picker-modal">
        <div class="picker-eyebrow">Qui a fait ça ?</div>
        <div class="picker-task-name"></div>
        <div class="people-row"></div>
        <button class="picker-cancel">Annuler</button>
      </div>
    `;
    overlay.addEventListener('click', e => { if (e.target === overlay) this._closePicker(); });
    overlay.querySelector('.picker-cancel').addEventListener('click', () => this._closePicker());
    root.appendChild(overlay);

    root.appendChild(card);
  }

  _taskRow(task, isDone) {
    const row = document.createElement('div');
    row.className = 'task-row';
    const ico = FOYER_WEEKLY_CAT_ICONS[task.cat] || '📌';
    const daysLeft = this._daysLeft(task.due);
    const target = task.weeklyTarget || 1;
    const count = task.weeklyCount || 0;
    const progress = target > 1 ? `<span class="badge badge-soon">${count}/${target}</span>` : '';

    let badge;
    if (isDone) {
      badge = target > 1 ? `<span class="badge badge-done">✓ ${count}/${target}</span>` : '<span class="badge badge-done">✓ Fait</span>';
    } else if (daysLeft <= 0) {
      badge = '<span class="badge badge-late">⚠ En retard</span>';
    } else if (daysLeft === 1) {
      badge = '<span class="badge badge-last">Dernier jour</span>';
    } else {
      badge = `<span class="badge badge-soon">${daysLeft}j restants</span>`;
    }

    const sub = task.lastDoneAt ? `<div class="task-sub">Dernière fois : ${this._fdate(task.lastDoneAt)}</div>` : '';

    row.innerHTML = `
      <div class="task-ico">${ico}</div>
      <div class="task-info">
        <div class="task-title">${this._esc(task.title)}</div>
        ${sub}
      </div>
      ${!isDone && target > 1 ? progress : ''}
      ${badge}
    `;

    if (!isDone) {
      row.addEventListener('click', () => this._openPicker(task));
    } else {
      row.style.cursor = 'default';
    }

    return row;
  }

  _openPicker(task) {
    this._pendingTask = task;
    const root = this.shadowRoot;
    root.querySelector('.picker-task-name').textContent = task.title;

    const peopleRow = root.querySelector('.people-row');
    peopleRow.innerHTML = this._members.map(m => `
      <button class="person-btn" data-member-id="${this._esc(m.id)}"
              style="--c: ${FOYER_WEEKLY_TONE_COLORS[m.tone] ?? '#888'}">
        <div class="person-avatar">${this._esc(m.initial)}</div>
        <span class="person-name">${this._esc(m.name)}</span>
      </button>
    `).join('');
    peopleRow.querySelectorAll('.person-btn').forEach(btn => {
      btn.addEventListener('click', e => {
        e.stopPropagation();
        this._completeTask(btn.dataset.memberId);
      });
    });

    root.querySelector('.picker-overlay').classList.add('open');
  }

  _closePicker() {
    this.shadowRoot.querySelector('.picker-overlay')?.classList.remove('open');
    this._pendingTask = null;
  }

  async _completeTask(memberId) {
    const task = this._pendingTask;
    if (!task) return;
    this._closePicker();
    try {
      const r = await fetch(`${this._apiBase}/api/tasks/${task.id}/complete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ memberId, at: new Date().toISOString() }),
      });
      if (!r.ok) throw new Error(r.status);
      await this._fetchData();
    } catch (e) {
      console.error('foyer-weekly-card: completeTask', e);
    }
  }

  _esc(s) {
    return String(s ?? '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  getCardSize() { return 3; }

  static getStubConfig() {
    return { title: 'Tâches hebdomadaires' };
  }
}

customElements.define('foyer-weekly-card', FoyerWeeklyCard);

window.customCards = window.customCards || [];
window.customCards.push({
  type: 'foyer-weekly-card',
  name: 'Foyer — Tâches hebdomadaires',
  description: "Tâches à faire une fois par semaine, n'importe quel jour",
});
