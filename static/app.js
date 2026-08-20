/* Four Depths — the only script on the site. */
(function () {
  var root = document.documentElement;
  root.classList.add('js');

  /* ---- theme: auto → light → dark ---- */
  var order = ['auto', 'light', 'dark'];
  var btn = document.getElementById('theme');
  if (btn) {
    var name = btn.querySelector('.theme-name');
    var show = function (t) {
      root.dataset.theme = t;
      name.textContent = t;
      btn.setAttribute('aria-label', 'Theme: ' + t + '. Press to change it.');
    };
    show(localStorage.getItem('theme') || 'auto');
    btn.addEventListener('click', function () {
      var next = order[(order.indexOf(root.dataset.theme) + 1) % order.length];
      try { localStorage.setItem('theme', next); } catch (e) {}
      show(next);
    });
  }

  /* ---- depth slider ---- */
  var box = document.getElementById('levels');
  if (!box) return;
  var levels = [].slice.call(box.querySelectorAll('.level'));
  var range = document.getElementById('range');
  /* a post can carry one depth only, and then it has no slider to drive */
  if (!range || levels.length < 2) {
    levels.forEach(function (el) {
      el.removeAttribute('hidden');
      el.setAttribute('aria-hidden', 'false');
    });
    return;
  }
  var now = document.getElementById('now');
  var stops = [].slice.call(document.querySelectorAll('#stops option'));
  var names = stops.map(function (o) { return o.label; });
  var sizes = stops.map(function (o) { return o.dataset.w + ' words · ' + o.dataset.r; });
  var label = now && now.querySelector('.lbl');
  var count = now && now.querySelector('.wc');
  var cur = 0;
  var busy = 0, gone = 0;

  /* A reader view or a clipper (Readability and everything built on it) does
     not look at opacity or at visibility: it takes whatever text is in the
     page. Without this, one clipping holds all five depths at once. These two
     attributes are what those tools honour, and inert keeps the text that is
     not on screen away from the keyboard and the screen reader as well. */
  /* A depth that is not on screen is hidden four ways, because a reader view
     or a clipper works on a copy of the page where the stylesheet is gone:
     only the attributes travel. hidden and an inline display cover the two
     tests those tools make, aria-hidden covers the third, and inert keeps the
     text away from the keyboard and the screen reader. */
  var sleep = function (el) {
    el.hidden = true;
    el.style.display = 'none';
    el.setAttribute('aria-hidden', 'true');
    if ('inert' in el) el.inert = true;
    /* and out of the page altogether. The attributes above are what a careful
       tool reads; this is what the rest cannot get around. The node is held in
       the levels array, so nothing is lost and nothing is parsed again. */
    if (el.parentNode) el.parentNode.removeChild(el);
  };
  var wake = function (el) {
    el.hidden = false;
    el.style.removeProperty('display');
    el.setAttribute('aria-hidden', 'false');
    if ('inert' in el) el.inert = false;
    /* every depth is placed by the stylesheet, not by its turn in the page, so
       it can go back at the end */
    if (!el.parentNode) box.appendChild(el);
  };
  /* every depth but the one on screen goes to sleep */
  var settle = function () {
    levels.forEach(function (el, i) { if (i === cur) { wake(el); } else { sleep(el); } });
  };

  /* ---- the depth the reader chose, kept between posts ---- */
  /* The choice is stored by name, not by position, because posts carry
     different sets of depths. A post that does not hold the chosen depth opens
     at the nearest one it has, and leaves the choice itself alone: only the
     reader moving the slider writes a new one. */
  var ORDER = ['sentence', 'paragraph', 'paragraphs', 'page', 'pages'];
  var keys = levels.map(function (el) { return el.dataset.k; });

  /* ---- the depth in the address ---- */
  /* The name in the hash is the one written in the markdown: #sentence,
     #paragraph, #short, #medium, #long. A copied link opens at that depth. A
     hash that is not one of the five is left alone, so a footnote link still
     works, and the stored choice applies instead. */
  var NAME_KEY = {
    sentence: 'sentence', sentance: 'sentence', paragraph: 'paragraph',
    short: 'paragraphs', medium: 'page', long: 'pages'
  };
  var KEY_NAME = {
    sentence: 'sentence', paragraph: 'paragraph',
    paragraphs: 'short', page: 'medium', pages: 'long'
  };
  /* the depth the post itself asks to open at, if it named one */
  var fromPage = function () {
    var d = (box.dataset.depth || '').toLowerCase();
    return Object.prototype.hasOwnProperty.call(NAME_KEY, d) ? NAME_KEY[d] : null;
  };
  var fromHash = function () {
    var h = (location.hash || '').replace(/^#/, '').toLowerCase();
    return Object.prototype.hasOwnProperty.call(NAME_KEY, h) ? NAME_KEY[h] : null;
  };
  var writeHash = function (i) {
    var name = KEY_NAME[keys[i]];
    if (!name || !history.replaceState) return;
    /* replaceState, so the back button leaves the post instead of stepping
       through every depth the reader touched, and the page does not jump */
    history.replaceState(null, '', '#' + name);
  };

  var chosen = function () {
    try { return localStorage.getItem('depth'); } catch (e) { return null; }
  };
  var remember = function (i) {
    try { localStorage.setItem('depth', keys[i]); } catch (e) {}
  };
  var nearest = function (key) {
    var aim = ORDER.indexOf(key);
    if (aim < 0) return 0;
    var best = 0, gap = Infinity;
    keys.forEach(function (k, i) {
      var d = Math.abs(ORDER.indexOf(k) - aim);
      if (d < gap) { gap = d; best = i; }   /* a tie keeps the shorter depth */
    });
    return best;
  };

  levels.forEach(function (el) { el.removeAttribute('hidden'); });

  var height = function (el) { return Math.ceil(el.getBoundingClientRect().height); };

  var lock = function (h) { box.style.height = h + 'px'; };

  var last = levels.length - 1;

  /* the thumb slides freely; only the fill follows it pixel by pixel */
  var paint = function (raw) {
    range.style.setProperty('--p', (raw / last) * 100 + '%');
  };
  var put = function (raw) { range.value = raw; paint(raw); };

  var mark = function (i) {
    if (label && label.textContent !== names[i]) {
      label.textContent = names[i];
      count.textContent = sizes[i];
      now.classList.remove('bump');
      void now.offsetWidth;
      now.classList.add('bump');
    }
  };

  /* ---- words ---- */

  /* One span per word, made once per depth, so each word can be timed. */
  function cutWords(level) {
    if (level.dataset.cut) return;
    level.dataset.cut = '1';
    var walk = document.createTreeWalker(level, NodeFilter.SHOW_TEXT, null);
    var nodes = [], n;
    while ((n = walk.nextNode())) {
      if (n.nodeValue.trim() && !n.parentNode.closest('pre')) nodes.push(n);
    }
    /* a picture or a code block is one piece, so it is timed like one word */
    [].slice.call(level.querySelectorAll('img, pre')).forEach(function (el) {
      el.classList.add('shot');
    });
    nodes.forEach(function (node) {
      var bag = document.createDocumentFragment();
      node.nodeValue.split(/(\s+)/).forEach(function (bit) {
        if (!bit) return;
        if (/^\s+$/.test(bit)) { bag.appendChild(document.createTextNode(bit)); return; }
        var span = document.createElement('span');
        span.className = 'w';
        span.textContent = bit;
        bag.appendChild(span);
      });
      node.parentNode.replaceChild(bag, node);
    });
  }

  /* Collect the words the reader can actually see. A wave nobody can see
     only makes the wait longer, so blocks off screen keep no delay at all. */
  function onScreen(level) {
    cutWords(level);
    var top = -140, foot = (window.innerHeight || 800) + 140, list = [];
    [].slice.call(level.children).forEach(function (block) {
      var r = block.getBoundingClientRect();
      if (r.bottom > top && r.top < foot) {
        block.classList.add('vis');
        list = list.concat([].slice.call(block.querySelectorAll('.w, .shot')));
        if (block.classList.contains('shot')) list.push(block);
      } else {
        block.classList.remove('vis');
      }
    });
    return list;
  }

  /* The middle of the shorter version. The words move out of this point, or
     collapse back into it, so the change reads from the inside out. */
  function heart(small) {
    var r = box.getBoundingClientRect();
    var tall = window.innerHeight || 800;
    return {
      x: r.left + r.width / 2,
      y: Math.max(70, Math.min(tall - 70, r.top + small / 2))
    };
  }

  /* Time each word mostly at random, with a light pull from that point, so
     the words scatter in and fill the text out instead of marching in rows.
     "inward" turns the pull around, for words on their way off the page. */
  var scatter = function (list, mid, span, inward, head) {
    var far = 1, gap = [];
    list.forEach(function (w) {
      var r = w.getBoundingClientRect();
      var dx = r.left + r.width / 2 - mid.x;
      var dy = r.top + r.height / 2 - mid.y;
      var d = Math.sqrt(dx * dx + dy * dy);
      gap.push(d);
      if (d > far) far = d;
    });
    list.forEach(function (w, i) {
      var pull = inward ? 1 - gap[i] / far : gap[i] / far;
      var k = 0.3 * pull + 0.7 * Math.random();
      w.style.setProperty('--d', (head + k * span).toFixed(1) + 'ms');
    });
    return head + span;
  };

  /* Longest common run of words, so the text can be edited into the next
     version instead of written out again. Only what is on screen is compared. */
  function shared(a, b) {
    var n = a.length, m = b.length;
    if (!n || !m || n * m > 400000) return [];
    var w = m + 1, T = new Uint16Array((n + 1) * w), i, j;
    for (i = n - 1; i >= 0; i--) {
      for (j = m - 1; j >= 0; j--) {
        T[i * w + j] = a[i] === b[j]
          ? T[(i + 1) * w + j + 1] + 1
          : Math.max(T[(i + 1) * w + j], T[i * w + j + 1]);
      }
    }
    var pairs = [];
    i = 0; j = 0;
    while (i < n && j < m) {
      if (a[i] === b[j]) { pairs.push([i, j]); i++; j++; }
      else if (T[(i + 1) * w + j] >= T[i * w + j + 1]) i++;
      else j++;
    }
    return pairs;
  }

  var odd = 0;
  var plain = function (w) {
    /* a picture or a code block never counts as the same thing twice */
    if (w.classList.contains('shot')) return '\u0000' + (odd++);
    return w.textContent.toLowerCase().replace(/[^a-z0-9]/g, '');
  };
  var boxOf = function (w) { return w.getBoundingClientRect(); };

  /* A word that both versions hold, and that has not moved far, keeps its
     place: it slides from where it was. Everything else is added or removed. */
  function hold(older, newer) {
    var oldBox = older.map(boxOf), newBox = newer.map(boxOf);
    var keptOld = [], keptNew = [];
    shared(older.map(plain), newer.map(plain)).forEach(function (pair) {
      var a = oldBox[pair[0]], b = newBox[pair[1]];
      var dx = a.left - b.left, dy = a.top - b.top;
      if (Math.abs(dx) > 460 || Math.abs(dy) > 460) return;
      var word = newer[pair[1]];
      word.style.setProperty('--dx', dx.toFixed(1) + 'px');
      word.style.setProperty('--dy', dy.toFixed(1) + 'px');
      word.classList.add('keep');
      older[pair[0]].classList.add('ghost');
      keptOld.push(pair[0]);
      keptNew.push(pair[1]);
    });
    return {
      out: older.filter(function (_, i) { return keptOld.indexOf(i) < 0; }),
      into: newer.filter(function (_, i) { return keptNew.indexOf(i) < 0; })
    };
  }

  var ANIM = ['pop', 'fall', 'is-leaving'];
  var strip = function (el) {
    ANIM.forEach(function (c) { el.classList.remove(c); });
    [].slice.call(el.querySelectorAll('.vis')).forEach(function (b) { b.classList.remove('vis'); });
    [].slice.call(el.querySelectorAll('.keep, .ghost')).forEach(function (w) {
      w.classList.remove('keep', 'ghost');
    });
  };

  function go(next) {
    next = Math.max(0, Math.min(levels.length - 1, next | 0));
    mark(next);
    if (next === cur) return;

    remember(next);
    writeHash(next);

    var out = levels[cur], into = levels[next];
    var grow = next > cur;

    clearTimeout(gone);
    levels.forEach(strip);        /* drop any animation still in flight */
    wake(into);                   /* it cannot be measured while it sleeps */
    void into.offsetWidth;        /* so a repeated move starts over */

    lock(height(out));
    cutWords(into);
    into.classList.add('is-active');
    var target = height(into);

    var mid = heart(Math.min(height(out), target));
    var edit = hold(onScreen(out), onScreen(into));
    var goneAt, doneAt;
    if (grow) {
      /* words are dropped and added at the same time, in no fixed order,
         until what is on the page matches the longer version */
      goneAt = scatter(edit.out, mid, 150, false, 0) + 160;
      doneAt = scatter(edit.into, mid, 380, false, 110) + 250;
    } else {
      /* the outer words tend to go first, so the text closes in on what it
         keeps, and the shorter version fills back in around it */
      goneAt = scatter(edit.out, mid, 190, true, 0) + 160;
      doneAt = scatter(edit.into, mid, 240, false, 150) + 250;
    }

    out.classList.remove('is-active');
    out.classList.add('is-leaving', 'fall');
    into.classList.add('pop');
    lock(target);

    cur = next;
    ends();

    /* the old text is dropped the moment its last word has closed up, so
       nothing has to be faded away */
    clearTimeout(gone);
    gone = setTimeout(function () {
      strip(out);
      if (out !== levels[cur]) sleep(out);
    }, goneAt);

    clearTimeout(busy);
    busy = setTimeout(function () {
      levels.forEach(function (el) { if (el !== levels[cur]) strip(el); });
      strip(levels[cur]);
      box.style.height = height(levels[cur]) + 'px';
      settle();
    }, doneAt);
  }

  /* slide anywhere, then settle on the nearest depth */
  var gliding = 0;
  function glide(from, to) {
    cancelAnimationFrame(gliding);
    /* A page nobody is looking at runs no animation frame, so the thumb would
       stay where it was and be wrong when the reader comes back. Put it where
       it belongs and skip the movement, which is only for the eye anyway. */
    if (document.hidden) { put(to); return; }
    var t0 = 0;
    var step = function (t) {
      t0 = t0 || t;
      var k = Math.min(1, (t - t0) / 190);
      put(from + (to - from) * (1 - Math.pow(1 - k, 3)));
      if (k < 1) gliding = requestAnimationFrame(step);
    };
    gliding = requestAnimationFrame(step);
  }
  var snap = function () {
    cancelAnimationFrame(gliding);
    if (+range.value !== cur) glide(+range.value, cur);
  };

  /* The text starts to change once the thumb is a third of the way to the
     next depth, so a drag shows its result at once. It falls back a little
     later than it moves on, which keeps a slow drag from flickering. */
  function aim(raw) {
    var n = cur;
    while (n < last && raw >= n + 0.32) n++;
    while (n > 0 && raw <= n - 0.78) n--;
    return n;
  }

  range.addEventListener('input', function () {
    cancelAnimationFrame(gliding);
    var raw = +range.value;
    paint(raw);
    go(aim(raw));
  });
  range.addEventListener('change', snap);
  range.addEventListener('pointerup', snap);
  range.addEventListener('pointercancel', snap);
  range.addEventListener('blur', snap);

  /* whole steps for the keyboard, whatever the slider allows the mouse */
  /* one move, from wherever it was asked for: a key, or one of the two steps */
  var moveTo = function (to) {
    to = Math.max(0, Math.min(last, to));
    if (to === cur) return;
    go(to);
    glide(+range.value, to);
  };

  var KEYS = { ArrowLeft: -1, ArrowDown: -1, ArrowRight: 1, ArrowUp: 1 };
  range.addEventListener('keydown', function (e) {
    var to = null;
    if (KEYS[e.key]) to = cur + KEYS[e.key];
    else if (e.key === 'Home') to = 0;
    else if (e.key === 'End') to = last;
    if (to === null) return;
    e.preventDefault();
    moveTo(to);
  });

  /* The two arrows work anywhere on the page, so a reader does not have to
     find the slider first. Up and down, home and end stay on the slider
     itself, because those keys scroll the page and are not ours to take. The
     slider's own handler runs first and stops the event, so a focused slider
     moves one depth, not two, and typing anywhere is left alone. */
  document.addEventListener('keydown', function (e) {
    if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey) return;
    if (e.key !== 'ArrowLeft' && e.key !== 'ArrowRight') return;
    var t = e.target;
    if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return;
    e.preventDefault();
    moveTo(cur + (e.key === 'ArrowLeft' ? -1 : 1));
  });

  /* the two steps, one depth at a time; each goes quiet at its own end */
  var shorter = document.getElementById('shorter');
  var longer = document.getElementById('longer');
  var ends = function () {
    if (shorter) shorter.disabled = cur === 0;
    if (longer) longer.disabled = cur === last;
  };
  if (shorter) shorter.addEventListener('click', function () { moveTo(cur - 1); });
  if (longer) longer.addEventListener('click', function () { moveTo(cur + 1); });

  /* A post opens at the depth in the address; then at the one the post itself
     asks for; then at the one the reader last chose. In every case it lands on
     the nearest depth this post carries. A post that names a depth therefore
     overrides the reader's own choice, but never a link they followed. */
  cur = nearest(fromHash() || fromPage() || chosen());
  box.classList.add('no-anim');
  levels.forEach(function (el, i) { el.classList.toggle('is-active', i === cur); });
  settle();
  ends();
  mark(cur);
  put(cur);
  lock(height(levels[cur]));
  requestAnimationFrame(function () { box.classList.remove('no-anim'); });

  /* coming back to the page: the thumb belongs at the depth on screen */
  document.addEventListener('visibilitychange', function () {
    if (!document.hidden) put(cur);
  });

  /* someone pastes a link, edits the address, or steps back to one */
  window.addEventListener('hashchange', function () {
    var key = fromHash();
    if (!key) return;
    var to = nearest(key);
    if (to === cur) return;
    go(to);
    glide(+range.value, to);
  });

  /* a resize changes how tall the text is */
  var timer;
  window.addEventListener('resize', function () {
    clearTimeout(timer);
    timer = setTimeout(function () {
      box.classList.add('no-anim');
      lock(height(levels[cur]));
      requestAnimationFrame(function () { box.classList.remove('no-anim'); });
    }, 120);
  });
})();

/* Four Depths — the lightbox.
   Its own block, because the script above stops early on a page that carries
   no depth slider, and a picture there must open just the same. */
(function () {
  var lb, pic, cap, shutBtn, opener;

  /* Every picture in a post is a control: it can be reached with the Tab key
     and opened with Enter or the space bar, not with a mouse only. */
  [].slice.call(document.querySelectorAll('.level img')).forEach(function (img) {
    img.tabIndex = 0;
    img.setAttribute('role', 'button');
    img.setAttribute('aria-label', 'See the picture full screen');
  });

  function build() {
    lb = document.createElement('div');
    lb.className = 'lb';
    lb.hidden = true;
    lb.setAttribute('role', 'dialog');
    lb.setAttribute('aria-modal', 'true');
    lb.innerHTML =
      '<button class="lb-x" type="button" aria-label="Close the picture">×</button>' +
      '<figure class="lb-body"><img class="lb-pic" alt=""><figcaption class="lb-cap"></figcaption></figure>';
    pic = lb.querySelector('.lb-pic');
    cap = lb.querySelector('.lb-cap');
    shutBtn = lb.querySelector('.lb-x');
    shutBtn.addEventListener('click', shut);
    /* the dark ground closes it; the picture and its caption do not */
    lb.addEventListener('click', function (e) {
      if (e.target === lb || e.target.classList.contains('lb-body')) shut();
    });
    document.body.appendChild(lb);
  }

  /* The caption is the italic line that follows the picture in the same
     paragraph. With none, the alt text says the same thing. */
  function captionOf(img) {
    var next = img.nextElementSibling;
    if (next && next.tagName === 'EM') return next.textContent;
    return img.getAttribute('alt') || '';
  }

  function open(img) {
    if (!lb) build();
    opener = img;
    pic.src = img.currentSrc || img.src;
    pic.alt = img.getAttribute('alt') || '';
    var text = captionOf(img);
    cap.textContent = text;
    cap.hidden = !text;
    lb.hidden = false;
    /* Holding the page still can take the scroll bar away, and the text would
       jump sideways into the space it leaves. Measure the space that opens up,
       and give exactly that much back. A browser whose scroll bar floats over
       the page takes none, and then this adds none. */
    var was = document.documentElement.clientWidth;
    document.documentElement.classList.add('lb-open');
    var bar = document.documentElement.clientWidth - was;
    document.body.style.paddingRight = bar > 0 ? bar + 'px' : '';
    shutBtn.focus();
  }

  function shut() {
    if (!lb || lb.hidden) return;
    lb.hidden = true;
    pic.removeAttribute('src');
    document.documentElement.classList.remove('lb-open');
    document.body.style.paddingRight = '';
    if (opener) opener.focus();
  }

  document.addEventListener('click', function (e) {
    var img = e.target.closest && e.target.closest('.level img');
    if (!img) return;
    e.preventDefault();
    open(img);
  });

  document.addEventListener('keydown', function (e) {
    var img = e.target.closest && e.target.closest('.level img');
    if (img && (e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault();
      open(img);
      return;
    }
    if (!lb || lb.hidden) return;
    if (e.key === 'Escape') { shut(); return; }
    /* one control inside, so the Tab key cannot leave the picture behind */
    if (e.key === 'Tab') { e.preventDefault(); shutBtn.focus(); }
  });
}());
