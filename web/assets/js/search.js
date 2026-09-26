// Navbar search suggestions.
//
// Typing calls the search API and lists a few hits under the input. Enter with
// no suggestion selected goes to the full results page; selecting a suggestion
// (click, or arrow keys then Enter) opens that post.
const MIN_QUERY_LENGTH = 2;
const DEBOUNCE_MS = 250;
const MAX_SUGGESTIONS = 6;

// Writes a server-built value into an element. Both strings the API returns for
// highlighting are ts_headline output over text the server escaped on the write
// path, so the only markup they can carry is the <mark> pair around the matching
// terms — assigning them as text would print those tags instead of highlighting.
// Falls back to plain text when there is no marked-up value.
function assignMarkup(element, markup, fallbackText) {
    if (markup) {
        element.innerHTML = markup;
        return;
    }
    element.textContent = fallbackText;
}

class SearchBox {
    constructor(input, panel) {
        this.input = input;
        this.panel = panel;
        this.items = [];
        this.activeIndex = -1;
        this.timer = null;
        this.controller = null;
        this.lastQuery = '';

        this.input.addEventListener('input', () => this.onInput());
        this.input.addEventListener('keydown', (event) => this.onKeyDown(event));
        // A click inside the panel must not be treated as "clicked outside".
        this.input.addEventListener('focus', () => {
            if (this.items.length > 0) {
                this.open();
            }
        });
        document.addEventListener('click', (event) => {
            if (!this.panel.contains(event.target) && event.target !== this.input) {
                this.close();
            }
        });
    }

    onInput() {
        const query = this.input.value.trim();

        clearTimeout(this.timer);
        if (query.length < MIN_QUERY_LENGTH) {
            this.abort();
            this.clear();
            this.close();
            return;
        }

        this.timer = setTimeout(() => this.search(query), DEBOUNCE_MS);
    }

    onKeyDown(event) {
        if (event.key === 'Escape') {
            this.close();
            return;
        }

        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            if (this.items.length === 0) {
                return;
            }
            event.preventDefault();
            const step = event.key === 'ArrowDown' ? 1 : -1;
            const count = this.items.length;
            this.activeIndex = (this.activeIndex + step + count) % count;
            this.highlight();
            return;
        }

        if (event.key === 'Enter') {
            // Deliberate: Enter only opens a post when one is actually picked
            // with the arrow keys. Otherwise it goes to the results page, so
            // typing a keyword and hitting Enter always lands somewhere useful.
            const active = this.items[this.activeIndex];
            if (active) {
                event.preventDefault();
                window.location.href = active.url;
                return;
            }

            const query = this.input.value.trim();
            if (query.length >= MIN_QUERY_LENGTH) {
                event.preventDefault();
                window.location.href = this.resultsURL(query);
            }
        }
    }

    async search(query) {
        this.lastQuery = query;
        this.abort();
        this.controller = new AbortController();

        let response;
        try {
            response = await fetch(
                `/api/v1/posts/search?q=${encodeURIComponent(query)}&offset=0`,
                { signal: this.controller.signal }
            );
        } catch (error) {
            // A cancelled request is the normal case while typing.
            if (error.name === 'AbortError') {
                return;
            }
            this.renderNote('Không tìm kiếm được, vui lòng thử lại.');
            return;
        }

        // A slower earlier request must not overwrite a newer one.
        if (query !== this.lastQuery) {
            return;
        }

        if (response.status === 429) {
            this.renderNote('Bạn thao tác quá nhanh, vui lòng thử lại sau.');
            return;
        }
        if (!response.ok) {
            this.renderNote('Không tìm kiếm được, vui lòng thử lại.');
            return;
        }

        const body = await response.json();
        this.render(query, body?.data ?? [], body?.total_count ?? 0);
    }

    render(query, posts, total) {
        // titleHTML and snippet are both ts_headline output over text the server
        // escaped on the write path, so the only markup they carry is the <mark>
        // pair around the matches. assignMarkup() below relies on that.
        this.items = posts.slice(0, MAX_SUGGESTIONS).map((post) => ({
            url: `/post/${post.id}`,
            title: post.title,
            titleHTML: post.title_snippet || '',
            snippet: post.snippet || '',
        }));

        this.panel.textContent = '';
        this.activeIndex = -1;

        if (this.items.length === 0) {
            this.renderNote('Không tìm thấy bài viết nào.');
            return;
        }

        this.items.forEach((item, index) => {
            const link = document.createElement('a');
            link.className = 'search-suggestion';
            link.href = item.url;
            link.id = `search-suggestion-${index}`;
            link.setAttribute('role', 'option');
            link.setAttribute('aria-selected', 'false');
            link.dataset.index = String(index);

            const title = document.createElement('span');
            title.className = 'search-suggestion-title';
            // A term that matched the title has no other place to show up, so the
            // marked-up title is preferred over the plain one.
            assignMarkup(title, item.titleHTML, item.title);

            const snippet = document.createElement('span');
            snippet.className = 'search-suggestion-snippet';
            assignMarkup(snippet, item.snippet, '');

            link.append(title, snippet);
            this.panel.appendChild(link);
        });

        const all = document.createElement('a');
        all.className = 'search-suggestion search-suggestion-all';
        all.id = 'search-suggestion-all';
        all.href = this.resultsURL(query);
        all.textContent = total > this.items.length
            ? `Xem tất cả ${total} kết quả cho “${query}”`
            : `Xem tất cả kết quả cho “${query}”`;
        this.panel.appendChild(all);

        this.open();
    }

    renderNote(message) {
        this.items = [];
        this.activeIndex = -1;
        this.panel.textContent = '';

        const note = document.createElement('div');
        note.className = 'search-suggestion-note';
        note.textContent = message;
        this.panel.appendChild(note);

        this.open();
    }

    highlight() {
        const links = this.panel.querySelectorAll('.search-suggestion');
        links.forEach((link) => {
            const isActive = Number(link.dataset.index) === this.activeIndex;
            link.classList.toggle('active', isActive);
            link.setAttribute('aria-selected', isActive ? 'true' : 'false');
        });

        const active = this.items[this.activeIndex];
        if (active) {
            this.input.setAttribute('aria-activedescendant', `search-suggestion-${this.activeIndex}`);
        } else {
            this.input.removeAttribute('aria-activedescendant');
        }
    }

    resultsURL(query) {
        return `/search?q=${encodeURIComponent(query)}`;
    }

    open() {
        this.panel.hidden = false;
        this.input.setAttribute('aria-expanded', 'true');
    }

    close() {
        this.panel.hidden = true;
        this.input.setAttribute('aria-expanded', 'false');
        this.input.removeAttribute('aria-activedescendant');
    }

    clear() {
        this.items = [];
        this.activeIndex = -1;
        this.panel.textContent = '';
    }

    abort() {
        if (this.controller) {
            this.controller.abort();
            this.controller = null;
        }
    }
}

const input = document.getElementById('navbar-search');
const panel = document.getElementById('search-suggestions');

if (input && panel) {
    new SearchBox(input, panel);
}

export default SearchBox;
