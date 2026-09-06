/**
 * callhook Documentation — Interactive Client Engine
 * Zero dependencies, pure vanilla JavaScript
 */

(function () {
  'use strict';

  // --- 1. THEME MANAGER ---
  const THEME_KEY = 'callhook_docs_theme';
  
  function getPreferredTheme() {
    try {
      const saved = localStorage.getItem(THEME_KEY);
      if (saved) return saved;
    } catch (e) {
      // Ignore storage restrictions on file:// or strict browser modes
    }
    return window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  }

  function applyTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    try {
      localStorage.setItem(THEME_KEY, theme);
    } catch (e) {
      // Ignore storage restrictions
    }
    updateThemeIcon(theme);
  }

  function updateThemeIcon(theme) {
    const btn = document.querySelector('.theme-toggle');
    if (!btn) return;
    if (theme === 'light') {
      btn.innerHTML = `<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"></path></svg>`;
      btn.setAttribute('aria-label', 'Switch to dark theme');
      btn.title = 'Switch to dark theme';
    } else {
      btn.innerHTML = `<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="5"></circle><line x1="12" y1="1" x2="12" y2="3"></line><line x1="12" y1="21" x2="12" y2="23"></line><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"></line><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"></line><line x1="1" y1="12" x2="3" y2="12"></line><line x1="21" y1="12" x2="23" y2="12"></line><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"></line><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"></line></svg>`;
      btn.setAttribute('aria-label', 'Switch to light theme');
      btn.title = 'Switch to light theme';
    }
  }

  // Initial theme application
  applyTheme(getPreferredTheme());

  document.addEventListener('DOMContentLoaded', () => {
    // Re-verify icon after DOM is loaded
    updateThemeIcon(document.documentElement.getAttribute('data-theme') || 'dark');
    
    const themeBtn = document.querySelector('.theme-toggle');
    if (themeBtn) {
      themeBtn.addEventListener('click', () => {
        const current = document.documentElement.getAttribute('data-theme') || 'dark';
        applyTheme(current === 'dark' ? 'light' : 'dark');
      });
    }

    // --- 2. CODE COPY BUTTON ENHANCEMENT ---
    const pres = document.querySelectorAll('pre');
    pres.forEach((pre) => {
      // If already wrapped, skip
      if (pre.parentElement && pre.parentElement.classList.contains('code-block-wrapper')) return;

      const wrapper = document.createElement('div');
      wrapper.className = 'code-block-wrapper';

      const header = document.createElement('div');
      header.className = 'code-block-header';

      // Infer language/type
      let lang = 'TERMINAL';
      const text = pre.textContent || '';
      if (text.trim().startsWith('{') || text.trim().startsWith('[')) {
        lang = 'JSON';
      } else if (text.includes('go run') || text.includes('curl ') || text.includes('$ ')) {
        lang = 'BASH';
      } else if (text.includes('package ') || text.includes('func ') || text.includes('map[string]')) {
        lang = 'GO';
      }

      const langTag = document.createElement('span');
      langTag.className = 'code-lang-tag';
      langTag.textContent = lang;

      const copyBtn = document.createElement('button');
      copyBtn.className = 'copy-btn';
      copyBtn.innerHTML = `<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg> Copy`;
      copyBtn.setAttribute('aria-label', 'Copy code snippet');

      copyBtn.addEventListener('click', async () => {
        const codeText = pre.innerText.replace(/^\$ /gm, ''); // Strip leading bash prompts
        try {
          await navigator.clipboard.writeText(codeText);
          copyBtn.classList.add('copied');
          copyBtn.innerHTML = `<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg> Copied!`;
          setTimeout(() => {
            copyBtn.classList.remove('copied');
            copyBtn.innerHTML = `<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg> Copy`;
          }, 2000);
        } catch (err) {
          console.error('Failed to copy text: ', err);
        }
      });

      header.appendChild(langTag);
      header.appendChild(copyBtn);

      pre.parentNode.insertBefore(wrapper, pre);
      wrapper.appendChild(header);
      wrapper.appendChild(pre);
    });

    // --- 3. MOBILE NAVIGATION DRAWER ---
    const mobileToggle = document.querySelector('.mobile-toggle');
    const mobileDrawer = document.querySelector('.mobile-drawer');
    const mobileOverlay = document.querySelector('.mobile-overlay');
    const drawerClose = document.querySelector('.drawer-close');

    function openDrawer() {
      if (mobileDrawer) mobileDrawer.classList.add('open');
      if (mobileOverlay) mobileOverlay.classList.add('open');
      document.body.style.overflow = 'hidden';
    }

    function closeDrawer() {
      if (mobileDrawer) mobileDrawer.classList.remove('open');
      if (mobileOverlay) mobileOverlay.classList.remove('open');
      document.body.style.overflow = '';
    }

    if (mobileToggle) mobileToggle.addEventListener('click', openDrawer);
    if (drawerClose) drawerClose.addEventListener('click', closeDrawer);
    if (mobileOverlay) mobileOverlay.addEventListener('click', closeDrawer);

    // --- 4. TABLE OF CONTENTS SCROLLSPY ---
    const tocLinks = document.querySelectorAll('.toc-link');
    const headings = Array.from(document.querySelectorAll('.docs-main h2, .docs-main h3'));

    if (tocLinks.length > 0 && headings.length > 0) {
      const observer = new IntersectionObserver((entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const id = entry.target.id;
            if (!id) return;
            tocLinks.forEach((link) => {
              if (link.getAttribute('href') === `#${id}`) {
                link.classList.add('active');
              } else {
                link.classList.remove('active');
              }
            });
          }
        });
      }, {
        rootMargin: '0px 0px -70% 0px',
        threshold: 0
      });

      headings.forEach((heading) => {
        if (heading.id) observer.observe(heading);
      });
    }

    // --- 5. PIPELINE VIEW TOGGLER ---
    const toggleBtns = document.querySelectorAll('.view-toggle-btn');
    toggleBtns.forEach((btn) => {
      btn.addEventListener('click', () => {
        const card = btn.closest('.pipeline-card');
        if (!card) return;
        const visual = card.querySelector('.visual-pipeline-grid');
        const raw = card.querySelector('.pipeline-raw');
        if (raw.style.display === 'none' || !raw.style.display) {
          raw.style.display = 'block';
          visual.style.display = 'none';
          btn.textContent = 'Switch to Visual Flow';
        } else {
          raw.style.display = 'none';
          visual.style.display = 'grid';
          btn.textContent = 'Switch to Raw ASCII';
        }
      });
    });

    // --- 6. COMMAND PALETTE / QUICK SEARCH (⌘K) ---
    initCommandPalette();
  });

  // Search items database across entire docs
  const searchIndex = [
    { title: 'Overview & Introduction', cat: 'Home', url: '/index.html' },
    { title: 'How It Works (Pipeline)', cat: 'Home', url: '/index.html#how-it-works' },
    { title: 'Not a robocall. Not another LLM.', cat: 'Home', url: '/index.html#features' },
    { title: 'Operational Guarantees', cat: 'Home', url: '/index.html#guarantees' },
    { title: 'Fire your first event', cat: 'Home', url: '/index.html#first-event' },
    
    { title: 'Quickstart: 1. Run callhook', cat: 'Quickstart', url: '/pages/quickstart.html#run-callhook' },
    { title: 'Quickstart: 2. Watch dashboard', cat: 'Quickstart', url: '/pages/quickstart.html#watch-dashboard' },
    { title: 'Quickstart: 3. Fire a real event', cat: 'Quickstart', url: '/pages/quickstart.html#fire-real-event' },
    { title: 'Quickstart: 4. Use the CLI (callhookctl)', cat: 'Quickstart', url: '/pages/quickstart.html#use-cli' },
    { title: 'Quickstart: 5. Going live', cat: 'Quickstart', url: '/pages/quickstart.html#going-live' },

    { title: 'POST /api/events', cat: 'API', url: '/pages/api.html#post-events' },
    { title: 'POST /api/events/batch', cat: 'API', url: '/pages/api.html#post-batch' },
    { title: 'POST /callhook/webhook', cat: 'API', url: '/pages/api.html#post-webhook' },
    { title: 'GET /api/sessions', cat: 'API', url: '/pages/api.html#get-sessions' },
    { title: 'GET /api/metrics', cat: 'API', url: '/pages/api.html#get-metrics' },
    { title: 'GET /api/health', cat: 'API', url: '/pages/api.html#get-health' },
    { title: 'Authentication (Bearer Token)', cat: 'API', url: '/pages/api.html#auth' },

    { title: 'Event Blueprint: invoice.due', cat: 'Events', url: '/pages/events.html#invoice-due' },
    { title: 'Event Blueprint: account.warning', cat: 'Events', url: '/pages/events.html#account-warning' },
    { title: 'Event Blueprint: promo.offer', cat: 'Events', url: '/pages/events.html#promo-offer' },
    { title: 'Adding your own Blueprint', cat: 'Events', url: '/pages/events.html#custom-events' },
    { title: 'Goals API integration', cat: 'Events', url: '/pages/events.html#goals-api' },

    { title: 'Production: 1. API key', cat: 'Production', url: '/pages/production.html#api-key' },
    { title: 'Production: 2. Public tunnel', cat: 'Production', url: '/pages/production.html#tunnel' },
    { title: 'Production: 3. Lock it down (Auth)', cat: 'Production', url: '/pages/production.html#auth' },
    { title: 'Production: 4. Persistence (JSONL)', cat: 'Production', url: '/pages/production.html#persistence' },
    { title: 'Configuration Reference', cat: 'Production', url: '/pages/production.html#config' },
    { title: 'Calling Behavior Defaults', cat: 'Production', url: '/pages/production.html#behavior' },
    { title: 'Connecting business.Store', cat: 'Production', url: '/pages/production.html#business-store' },

    { title: 'Architecture: The pipeline', cat: 'Architecture', url: '/pages/architecture.html#pipeline' },
    { title: 'Architecture: Packages overview', cat: 'Architecture', url: '/pages/architecture.html#packages' },
    { title: 'Design Decisions (Read/write, No LLM)', cat: 'Architecture', url: '/pages/architecture.html#design-decisions' },
    { title: 'Testing Suites', cat: 'Architecture', url: '/pages/architecture.html#testing' }
  ];

  function initCommandPalette() {
    // Determine relative root prefix
    const isSubpage = window.location.pathname.includes('/pages/');
    const rootPrefix = isSubpage ? '../' : '';

    const backdrop = document.createElement('div');
    backdrop.className = 'cmd-palette-backdrop';
    backdrop.innerHTML = `
      <div class="cmd-palette-modal" role="dialog" aria-modal="true">
        <div class="cmd-palette-search-bar">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
          <input type="text" class="cmd-palette-input" placeholder="Search documentation, endpoints, guides..." autofocus />
          <span class="cmd-palette-esc">ESC</span>
        </div>
        <div class="cmd-palette-results"></div>
      </div>
    `;

    document.body.appendChild(backdrop);

    const input = backdrop.querySelector('.cmd-palette-input');
    const resultsContainer = backdrop.querySelector('.cmd-palette-results');
    let selectedIndex = 0;
    let currentMatches = [];

    function renderResults(query = '') {
      const q = query.toLowerCase().trim();
      currentMatches = searchIndex.filter(item => {
        return !q || item.title.toLowerCase().includes(q) || item.cat.toLowerCase().includes(q);
      });

      if (currentMatches.length === 0) {
        resultsContainer.innerHTML = `<div style="padding: 24px; text-align: center; color: var(--text-tertiary); font-size: 13.5px;">No matching documentation found.</div>`;
        return;
      }

      resultsContainer.innerHTML = currentMatches.map((item, idx) => {
        let cleanUrl = item.url;
        if (cleanUrl.startsWith('/')) cleanUrl = cleanUrl.substring(1);
        const resolvedUrl = rootPrefix + cleanUrl;
        return `
          <a href="${resolvedUrl}" class="cmd-item ${idx === selectedIndex ? 'selected' : ''}" data-idx="${idx}">
            <span class="item-title">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline></svg>
              ${item.title}
            </span>
            <span class="item-category">${item.cat}</span>
          </a>
        `;
      }).join('');
    }

    function openPalette() {
      backdrop.classList.add('open');
      input.value = '';
      selectedIndex = 0;
      renderResults();
      setTimeout(() => input.focus(), 50);
      document.body.style.overflow = 'hidden';
    }

    function closePalette() {
      backdrop.classList.remove('open');
      document.body.style.overflow = '';
    }

    // Search button click handler
    document.querySelectorAll('.search-btn').forEach(btn => {
      btn.addEventListener('click', openPalette);
    });

    // Keyboard trigger: Cmd+K / Ctrl+K / Escape
    window.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        if (backdrop.classList.contains('open')) {
          closePalette();
        } else {
          openPalette();
        }
      } else if (e.key === 'Escape' && backdrop.classList.contains('open')) {
        closePalette();
      } else if (backdrop.classList.contains('open')) {
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          if (selectedIndex < currentMatches.length - 1) {
            selectedIndex++;
            renderResults(input.value);
          }
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          if (selectedIndex > 0) {
            selectedIndex--;
            renderResults(input.value);
          }
        } else if (e.key === 'Enter') {
          e.preventDefault();
          if (currentMatches[selectedIndex]) {
            let cleanUrl = currentMatches[selectedIndex].url;
            if (cleanUrl.startsWith('/')) cleanUrl = cleanUrl.substring(1);
            window.location.href = rootPrefix + cleanUrl;
          }
        }
      }
    });

    input.addEventListener('input', (e) => {
      selectedIndex = 0;
      renderResults(e.target.value);
    });

    backdrop.addEventListener('click', (e) => {
      if (e.target === backdrop) closePalette();
    });
  }

})();
