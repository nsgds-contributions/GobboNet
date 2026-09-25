/*
 * The dashboard is rebuilt with innerHTML by renderMessages() on every 5 s
 * status poll and every card click (js/24-boot.js, js/15-cards.js). On a wide
 * screen YOUR CHARACTERS and the schedule list scroll inside their own boxes,
 * so each rebuild handed back fresh boxes at the top: the list "jumped" while
 * the user was scrolling it. renderMessages() now carries those positions
 * across the rebuild, and a dashboard opened fresh still starts at the top.
 *
 * The fake container behaves like the real one where it matters: assigning
 * innerHTML replaces every list with a new element whose scrollTop is 0.
 */
import fs from 'node:fs';
import vm from 'node:vm';

const SRC = fs.readFileSync(new URL('../js/13-dashboard.js', import.meta.url), 'utf8');
const a = SRC.indexOf('function renderMessages()');
const b = SRC.indexOf('\nfunction ', a + 1);
if (a < 0 || b < 0) {
  console.error('could not locate renderMessages() in js/13-dashboard.js');
  process.exit(1);
}
const FN = SRC.slice(a, b);

let pass = 0, fail = 0;
const eq = (got, want, msg) => {
  if (got === want) { pass++; console.log('  ok   ' + msg); }
  else { fail++; console.log(`  FAIL ${msg}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`); }
};

function makeContainer() {
  const c = { lists: [], builds: 0 };
  Object.defineProperty(c, 'innerHTML', {
    set(html) {
      c.builds++;
      // A landing page has the schedule list, YOUR CHARACTERS and the default
      // characters, in that order; anything else (a thread) has none.
      c.lists = html === 'LANDING'
        ? ['landing-sched-list', 'landing-char-list', 'landing-char-list'].map(cls => ({ cls, scrollTop: 0, build: c.builds }))
        : [];
    },
  });
  c.querySelectorAll = sel => c.lists.filter(l => sel.includes('.' + l.cls));
  return c;
}

function harness() {
  const container = makeContainer();
  const ctx = {
    document: {
      getElementById: id => id === 'messages' ? container
        : id === 'thread-title' ? { textContent: '' }
        : { classList: { add() {}, remove() {} } },
    },
    getActiveThread: () => null,
    renderLandingPage: () => 'LANDING',
    Array,
  };
  vm.createContext(ctx);
  vm.runInContext(FN, ctx);
  return { container, render: () => vm.runInContext('renderMessages()', ctx) };
}

console.log('A. a dashboard opened fresh starts at the top');
let h = harness();
h.container.innerHTML = 'THREAD';
h.render();
eq(h.container.lists.map(l => l.scrollTop).join(','), '0,0,0', 'every list at the top');

console.log('B. the 5 s poll / a card click rebuilds the page and keeps the place');
h.container.lists[1].scrollTop = 300;
h.container.lists[0].scrollTop = 120;
const before = h.container.lists[1];
h.render();
eq(h.container.lists[1] !== before, true, 'the list really is a new element');
eq(h.container.lists[1].scrollTop, 300, 'YOUR CHARACTERS keeps its position');
eq(h.container.lists[0].scrollTop, 120, 'the schedule list keeps its position');
eq(h.container.lists[2].scrollTop, 0, 'an unscrolled list stays at the top');

console.log('C. repeated rebuilds do not drift');
h.render(); h.render();
eq(h.container.lists[1].scrollTop, 300, 'still 300 after two more rebuilds');

console.log(`\n${pass} passed, ${fail} failed`);
process.exit(fail ? 1 : 0);
