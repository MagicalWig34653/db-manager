// SPDX-License-Identifier: AGPL-3.0-only
const form = document.querySelector('#search-form');
if (form) {
  const input = document.querySelector('#search-input');
  const results = document.querySelector('#search-results');
  let index;
  let epoch = 0;
  form.addEventListener('submit', async (event) => {
    event.preventDefault();
    const current = ++epoch;
    const terms = input.value.trim().toLocaleLowerCase('de').split(/\s+/).filter(Boolean);
    results.replaceChildren();
    if (!terms.length) return;
    try {
      index ??= fetch('/db-manager/search/search_index.json').then(response => {
        if (!response.ok) throw new Error('search');
        return response.json();
      }).then(value => value.docs);
      const docs = await index;
      if (current !== epoch) return;
      const matches = docs.filter(doc => terms.every(term => `${doc.title} ${doc.text}`.toLocaleLowerCase('de').includes(term))).slice(0, 40);
      const status = document.createElement('p');
      status.textContent = matches.length ? `${matches.length} Treffer (höchstens 40 angezeigt)` : 'Keine Treffer. Bitte andere Suchbegriffe versuchen.';
      results.append(status);
      for (const doc of matches) {
        const location = new URL(doc.location, `${locationOrigin()}/db-manager/`);
        if (location.origin !== window.location.origin || !location.pathname.startsWith('/db-manager/')) continue;
        const article = document.createElement('article');
        const link = document.createElement('a');
        link.href = location.href;
        link.textContent = doc.title;
        const excerpt = document.createElement('p');
        excerpt.textContent = doc.text.replace(/\s+/g, ' ').slice(0, 230) + ' …';
        article.append(link, excerpt);
        results.append(article);
      }
    } catch {
      index = undefined;
      results.textContent = 'Suche derzeit nicht verfügbar. Die Dokumentation bleibt über die Navigation erreichbar.';
    }
  });
}
function locationOrigin() { return window.location.origin; }
