// Tiny markdown -> HTML converter for the registry view files.
// We don't pull in a 200KB library just to render the section
// headings / lists / links the view generator emits. The grammar
// we support is exactly the subset the view generator produces:
// # / ## / ### headings, - bullet lists, 1. numbered lists,
// **bold**, [text](url) links, and `inline code`.
//
// Unsafe-by-design: the source files are produced by our own
// crawler, so we trust them. If untrusted markdown is ever
// served, switch to react-markdown.

export function renderMarkdown(src: string): string {
  const lines = src.split('\n');
  const out: string[] = [];
  let inList = false;
  let inOl = false;
  let inCode = false;
  let codeBuf: string[] = [];

  const flushLists = () => {
    if (inList) {
      out.push('</ul>');
      inList = false;
    }
    if (inOl) {
      out.push('</ol>');
      inOl = false;
    }
  };

  for (const raw of lines) {
    const line = raw;

    if (line.startsWith('```')) {
      if (inCode) {
        out.push('<pre><code>' + escapeHtml(codeBuf.join('\n')) + '</code></pre>');
        codeBuf = [];
        inCode = false;
      } else {
        flushLists();
        inCode = true;
      }
      continue;
    }
    if (inCode) {
      codeBuf.push(line);
      continue;
    }

    // Headings
    const h = /^(#{1,6})\s+(.*)$/.exec(line);
    if (h) {
      flushLists();
      const level = h[1]!.length;
      out.push(`<h${level}>${inline(h[2]!)}</h${level}>`);
      continue;
    }

    // Unordered list
    const ul = /^[-*]\s+(.*)$/.exec(line);
    if (ul) {
      if (!inList) {
        flushLists();
        out.push('<ul>');
        inList = true;
      }
      out.push(`<li>${inline(ul[1]!)}</li>`);
      continue;
    }

    // Ordered list
    const ol = /^\d+\.\s+(.*)$/.exec(line);
    if (ol) {
      if (!inOl) {
        flushLists();
        out.push('<ol>');
        inOl = true;
      }
      out.push(`<li>${inline(ol[1]!)}</li>`);
      continue;
    }

    // Blank line
    if (line.trim() === '') {
      flushLists();
      continue;
    }

    flushLists();
    out.push(`<p>${inline(line)}</p>`);
  }

  flushLists();
  if (inCode) {
    out.push('<pre><code>' + escapeHtml(codeBuf.join('\n')) + '</code></pre>');
  }
  return out.join('\n');
}

function inline(s: string): string {
  return escapeHtml(s)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>');
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}
