import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkRehype from "remark-rehype";
import rehypeSlug from "rehype-slug";
import rehypeShiki from "@shikijs/rehype";
import rehypeStringify from "rehype-stringify";
import { visit, SKIP } from "unist-util-visit";
import type { Root as Mdast, PhrasingContent, Parent, Text } from "mdast";
import type { Root as Hast, Element, Node as HastNode } from "hast";
import { row } from "./vault.ts";

export type Ref = { key: string; label: string; type: string; status: string | null; resolved: boolean };

export function resolve(target: string): Ref {
  const [name, alias] = target.split("|");
  const key = name.split("#")[0].trim();
  const r = row(key);
  return {
    key,
    label: alias?.trim() || r?.title || key,
    type: r?.type ?? key.split(".")[0],
    status: r?.status ?? null,
    resolved: !!r,
  };
}

const wikiRe = /\[\[([^[\]\n]+?)\]\]/g;
const safeUrl = /^(https?:|mailto:|\/|#)/i;

// The body is untrusted: raw HTML becomes text, and a link or image whose URL
// is not http(s), mailto, a path or an anchor loses the URL (`javascript:` and
// `data:` would otherwise reach the page as live markup).
function remarkSafe() {
  return (tree: Mdast) => {
    visit(tree, "html", (node, i, parent) => {
      parent!.children[i!] = { type: "text", value: node.value };
    });
    visit(tree, (n) => n.type === "link" || n.type === "image" || n.type === "definition", (node, i, parent) => {
      const n = node as { type: string; url: string; alt?: string | null; children?: PhrasingContent[] };
      if (safeUrl.test(n.url)) return;
      if (n.type === "definition") { n.url = "#"; return; }
      const kids: PhrasingContent[] = n.type === "image" ? [{ type: "text", value: n.alt ?? "" }] : n.children ?? [];
      (parent as Parent).children.splice(i!, 1, ...kids);
      return [SKIP, i!];
    });
  };
}

// Splits `[[key|label]]` out of text nodes. Code spans and blocks are separate
// node types, so a wikilink quoted as code stays code.
function remarkWikilinks() {
  return (tree: Mdast) => {
    visit(tree, "text", (node: Text, i, parent) => {
      if (!node.value.includes("[[") || !parent || i === undefined) return;
      const out: PhrasingContent[] = [];
      let last = 0;
      for (const m of node.value.matchAll(wikiRe)) {
        if (m.index > last) out.push({ type: "text", value: node.value.slice(last, m.index) });
        const ref = resolve(m[1]);
        out.push(
          ref.resolved
            ? {
                type: "link",
                url: `/a/${ref.key}`,
                children: [{ type: "text", value: ref.label }],
                data: { hProperties: { className: ["wl", `s-${ref.status ?? "none"}`], "data-type": ref.type } },
              }
            : {
                type: "text",
                value: ref.label,
                data: { hName: "span", hProperties: { className: ["wl-missing"], title: "unresolved link" } },
              },
        );
        last = m.index + m[0].length;
      }
      if (last < node.value.length) out.push({ type: "text", value: node.value.slice(last) });
      parent.children.splice(i, 1, ...out);
      return [SKIP, i + out.length];
    });
  };
}

export type TocEntry = { depth: number; text: string; id: string };

const toText = (n: HastNode): string =>
  ("value" in n ? String(n.value) : "") + ("children" in n ? (n.children as HastNode[]).map(toText).join("") : "");

const processor = unified()
  .use(remarkParse)
  .use(remarkGfm)
  .use(remarkSafe)
  .use(remarkWikilinks)
  .use(remarkRehype)
  .use(rehypeSlug)
  .use(rehypeShiki, { theme: "github-dark-default", defaultLanguage: "text", fallbackLanguage: "text" })
  .use(rehypeStringify);

export async function render(md: string): Promise<{ html: string; toc: TocEntry[] }> {
  const hast = (await processor.run(processor.parse(md))) as Hast;
  const toc: TocEntry[] = [];
  visit(hast, "element", (el: Element) => {
    if (el.tagName === "h2" || el.tagName === "h3")
      toc.push({ depth: Number(el.tagName[1]), text: toText(el), id: String(el.properties.id) });
  });
  return { html: processor.stringify(hast), toc };
}
