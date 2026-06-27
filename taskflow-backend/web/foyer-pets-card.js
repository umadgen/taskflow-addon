// foyer-pets-card — Lovelace custom card for pet health tracking
// Fetches directly from the Taskflow backend API (no HA sensor needed)

const FOYER_PETS_SPECIES_EMOJI = {
  chien: '🐕', chat: '🐈', lapin: '🐇', oiseau: '🐦',
  poisson: '🐠', reptile: '🦎', autre: '🐾',
};

const FOYER_PETS_CSS = `
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :host { display: block; }
  ha-card {
    background: var(--card-background-color);
    border-radius: var(--ha-card-border-radius, 12px);
    overflow: hidden;
    padding-bottom: 8px;
  }
  .card-header {
    padding: 16px 16px 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--primary-text-color);
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .pet-section {
    margin: 12px 12px 4px;
    border: 1px solid var(--divider-color, #e5e7eb);
    border-radius: 10px;
    overflow: hidden;
  }
  .pet-head {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    background: var(--secondary-background-color, #f9fafb);
    border-bottom: 1px solid var(--divider-color, #e5e7eb);
  }
  .pet-emoji { font-size: 28px; line-height: 1; flex-shrink: 0; }
  .pet-name { font-size: 15px; font-weight: 700; color: var(--primary-text-color); }
  .pet-meta { font-size: 12px; color: var(--secondary-text-color); margin-top: 1px; }
  .pet-weight { margin-left: auto; font-size: 13px; color: var(--secondary-text-color); }
  .item-list { padding: 4px 0; }
  .item-row {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 12px;
    border-bottom: 1px solid var(--divider-color, #f0f0f0);
  }
  .item-row:last-child { border-bottom: none; }
  .item-ico { font-size: 16px; flex-shrink: 0; width: 22px; text-align: center; }
  .item-info { flex: 1; min-width: 0; }
  .item-label { font-size: 13px; font-weight: 500; color: var(--primary-text-color); }
  .item-sub { font-size: 11px; color: var(--secondary-text-color); margin-top: 1px; }
  .badge {
    font-size: 11px; font-weight: 600; padding: 2px 7px;
    border-radius: 999px; white-space: nowrap; flex-shrink: 0;
  }
  .badge-late { background: #fef3c7; color: #92400e; }
  .badge-soon { background: #fff7ed; color: #c2410c; }
  .badge-ok   { background: #d1fae5; color: #065f46; }
  .badge-future { background: #eff6ff; color: #1d4ed8; }
  .btn-done {
    flex-shrink: 0; padding: 4px 9px; border-radius: 6px;
    border: none; background: var(--success-color, #16a34a); color: #fff;
    font-size: 11px; font-weight: 600; cursor: pointer;
    transition: opacity .15s;
  }
  .btn-done:hover { opacity: .8; }
  .empty-pet {
    padding: 8px 12px;
    font-size: 12px;
    color: var(--secondary-text-color);
    font-style: italic;
  }
  .card-empty {
    padding: 32px 16px;
    text-align: center;
    font-size: 14px;
    color: var(--secondary-text-color);
  }
  .card-empty .ico { font-size: 40px; margin-bottom: 8px; }
  .error { padding: 12px 16px; color: var(--error-color, #dc2626); font-size: 13px; }
`;

class FoyerPetsCard extends HTMLElement {
  constructor() {
    super();
    this.attachShadow({ mode: 'open' });
    this._pets = [];
    this._loading = true;
    this._error = null;
    this._pollTimer = null;
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
    this._fetchPets();
    this._pollTimer = setInterval(() => this._fetchPets(), 30000);
  }

  async _fetchPets() {
    try {
      const r = await fetch(`${this._apiBase}/api/pets`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      this._pets = await r.json();
      this._error = null;
      this._loading = false;
    } catch (e) {
      this._error = e.message;
      this._loading = false;
    }
    this._render();
  }

  _render() {
    const root = this.shadowRoot;
    root.innerHTML = '';

    const style = document.createElement('style');
    style.textContent = FOYER_PETS_CSS;
    root.appendChild(style);

    const card = document.createElement('ha-card');

    const title = this._config?.title || 'Animaux';
    card.innerHTML = `<div class="card-header">🐾 ${title}</div>`;

    if (this._loading) {
      card.innerHTML += '<div class="card-empty"><p>Chargement…</p></div>';
      root.appendChild(card);
      return;
    }

    if (this._error) {
      card.innerHTML += `<div class="error">⚠ Impossible de contacter le backend : ${this._error}</div>`;
      root.appendChild(card);
      return;
    }

    if (!this._pets || !this._pets.length) {
      card.innerHTML += '<div class="card-empty"><div class="ico">🐾</div><p>Aucun animal enregistré.<br>Ajoutez-en via l\'interface admin.</p></div>';
      root.appendChild(card);
      return;
    }

    const today = new Date().toISOString().slice(0, 10);
    const horizon = this._config?.horizon_days ?? 30;
    const horizonDate = new Date();
    horizonDate.setDate(horizonDate.getDate() + horizon);
    const horizonStr = horizonDate.toISOString().slice(0, 10);

    this._pets.forEach(pet => {
      const section = document.createElement('div');
      section.className = 'pet-section';

      const emoji = pet.emoji || FOYER_PETS_SPECIES_EMOJI[pet.species] || '🐾';
      const meta = [pet.species, pet.breed, pet.age, pet.sex === 'M' ? 'Mâle' : pet.sex === 'F' ? 'Femelle' : ''].filter(Boolean).join(' · ');
      const weightStr = pet.weight ? `${pet.weight} kg` : '';

      section.innerHTML = `
        <div class="pet-head">
          <div class="pet-emoji">${emoji}</div>
          <div>
            <div class="pet-name">${this._esc(pet.name)}</div>
            ${meta ? `<div class="pet-meta">${this._esc(meta)}</div>` : ''}
          </div>
          ${weightStr ? `<div class="pet-weight">⚖ ${weightStr}</div>` : ''}
        </div>
        <div class="item-list" id="items-${pet.id}"></div>
      `;

      root.appendChild(document.createTextNode('')); // flush
      card.appendChild(section);

      const list = section.querySelector(`#items-${pet.id}`);
      let hasItems = false;

      // Vaccines
      (pet.vaccines || []).forEach(v => {
        if (!v.next) return;
        if (v.next > horizonStr && v.next >= today) return; // too far
        const days = this._daysDiff(today, v.next);
        hasItems = true;
        const row = this._makeItemRow(
          '💉', v.label,
          `Rappel : ${this._fdate(v.next)}`,
          this._dateBadge(v.next, today, days)
        );
        if (days >= 0) {
          const btn = document.createElement('button');
          btn.className = 'btn-done';
          btn.textContent = '✓ Fait';
          btn.onclick = () => this._completeVaccine(pet.id, v.id);
          row.appendChild(btn);
        }
        list.appendChild(row);
      });

      // Treatments
      (pet.treatments || []).forEach(t => {
        if (!t.next) return;
        if (t.next > horizonStr && t.next >= today) return;
        const days = this._daysDiff(today, t.next);
        hasItems = true;
        const row = this._makeItemRow(
          '💊', t.label,
          `${t.every} · Prochain : ${this._fdate(t.next)}`,
          this._dateBadge(t.next, today, days)
        );
        if (days >= -3) { // allow completing slightly late
          const btn = document.createElement('button');
          btn.className = 'btn-done';
          btn.textContent = '✓ Fait';
          btn.onclick = () => this._completeTreatment(pet.id, t.id);
          row.appendChild(btn);
        }
        list.appendChild(row);
      });

      // Vet appointments (upcoming only)
      (pet.vet || [])
        .filter(v => v.date >= today)
        .sort((a, b) => a.date > b.date ? 1 : -1)
        .forEach(v => {
          const days = this._daysDiff(today, v.date);
          hasItems = true;
          const sub = [this._fdate(v.date), v.time, v.clinic].filter(Boolean).join(' · ');
          const row = this._makeItemRow('🏥', v.label, sub, this._dateBadge(v.date, today, days));
          list.appendChild(row);
        });

      if (!hasItems) {
        list.innerHTML = '<div class="empty-pet">Aucune urgence dans les ' + horizon + ' prochains jours ✓</div>';
      }
    });

    root.appendChild(card);
  }

  _makeItemRow(ico, label, sub, badge) {
    const row = document.createElement('div');
    row.className = 'item-row';
    row.innerHTML = `
      <div class="item-ico">${ico}</div>
      <div class="item-info">
        <div class="item-label">${this._esc(label)}</div>
        <div class="item-sub">${this._esc(sub)}</div>
      </div>
      ${badge}
    `;
    return row;
  }

  _dateBadge(date, today, days) {
    if (date < today) return `<span class="badge badge-late">⚠ ${Math.abs(days)}j retard</span>`;
    if (days === 0)   return `<span class="badge badge-soon">Aujourd'hui</span>`;
    if (days <= 7)    return `<span class="badge badge-soon">Dans ${days}j</span>`;
    if (days <= 30)   return `<span class="badge badge-future">Dans ${days}j</span>`;
    return `<span class="badge badge-ok">${this._fdate(date)}</span>`;
  }

  async _completeTreatment(petId, treatId) {
    try {
      const today = new Date().toISOString().slice(0, 10);
      const r = await fetch(`${this._apiBase}/api/pets/${petId}/treatments/${treatId}/complete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ today }),
      });
      if (!r.ok) throw new Error(r.status);
      await this._fetchPets();
    } catch (e) {
      console.error('foyer-pets-card: completeTreatment', e);
    }
  }

  async _completeVaccine(petId, vaccId) {
    try {
      const today = new Date().toISOString().slice(0, 10);
      const next = new Date();
      next.setFullYear(next.getFullYear() + 1);
      const r = await fetch(`${this._apiBase}/api/pets/${petId}/vaccines/${vaccId}/complete`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ today, next: next.toISOString().slice(0, 10) }),
      });
      if (!r.ok) throw new Error(r.status);
      await this._fetchPets();
    } catch (e) {
      console.error('foyer-pets-card: completeVaccine', e);
    }
  }

  _daysDiff(from, to) {
    return Math.round((new Date(to) - new Date(from)) / 864e5);
  }

  _fdate(iso) {
    if (!iso) return '';
    try {
      return new Date(iso.length === 10 ? iso + 'T12:00:00' : iso)
        .toLocaleDateString('fr-FR', { day: '2-digit', month: 'short', year: undefined });
    } catch { return iso; }
  }

  _esc(s) {
    return String(s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  getCardSize() { return 3; }

  static getConfigElement() {
    return document.createElement('foyer-pets-card-editor');
  }

  static getStubConfig() {
    return { title: 'Animaux', horizon_days: 30 };
  }
}

customElements.define('foyer-pets-card', FoyerPetsCard);

window.customCards = window.customCards || [];
window.customCards.push({
  type: 'foyer-pets-card',
  name: 'Foyer — Animaux',
  description: 'Suivi santé des animaux (vaccins, traitements, RDV véto)',
});
