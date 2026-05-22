(function () {
  const el = (id) => document.getElementById(id);
  const listEl = el('list');
  const adminMsg = el('adminMsg');
  let authed = document.body.dataset.authed === 'true';

  function esc(value) {
    return String(value || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function card(file) {
    const date = new Date(file.addedAt).toLocaleString('fr-FR');
    return [
      '<article class="card">',
      '<h3>' + esc(file.title) + '</h3>',
      '<p>' + esc(file.description || 'Aucune description.') + '</p>',
      '<div class="meta">Ajouté le ' + esc(date) + '</div>',
      '<a href="' + esc(file.url) + '" target="_blank" rel="noopener" class="download">Télécharger</a>',
      '</article>'
    ].join('');
  }

  async function loadFiles() {
    const res = await fetch('/api/files');
    const data = await res.json();
    const files = data.files || [];
    listEl.innerHTML = files.length ? files.map(card).join('') : '<p class="empty">Aucun fichier pour le moment.</p>';
  }

  function syncAdminUI() {
    el('authBox').classList.toggle('hidden', authed);
    el('addBox').classList.toggle('hidden', !authed);
  }

  async function login() {
    const user = el('user').value;
    const pass = el('pass').value;
    const res = await fetch('/api/admin/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user: user, pass: pass })
    });
    if (!res.ok) {
      adminMsg.textContent = 'Identifiants invalides.';
      return;
    }
    authed = true;
    adminMsg.textContent = 'Connecté.';
    syncAdminUI();
  }

  async function logout() {
    await fetch('/api/admin/logout', { method: 'POST' });
    authed = false;
    adminMsg.textContent = 'Déconnecté.';
    syncAdminUI();
  }

  async function addFile() {
    const title = el('title').value.trim();
    const url = el('url').value.trim();
    const description = el('desc').value.trim();
    const res = await fetch('/api/admin/files', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: title, url: url, description: description })
    });
    if (!res.ok) {
      adminMsg.textContent = 'Ajout impossible (vérifie le lien ou la session).';
      return;
    }
    adminMsg.textContent = 'Fichier ajouté.';
    el('title').value = '';
    el('url').value = '';
    el('desc').value = '';
    await loadFiles();
  }

  el('adminToggle').addEventListener('click', function () {
    el('adminPanel').classList.toggle('hidden');
    syncAdminUI();
  });
  el('loginBtn').addEventListener('click', login);
  el('logoutBtn').addEventListener('click', logout);
  el('addBtn').addEventListener('click', addFile);
  el('refreshBtn').addEventListener('click', loadFiles);

  syncAdminUI();
  loadFiles();
})();
