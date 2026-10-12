import { unified } from "unified";
import remarkParse from "remark-parse";
import remarkGfm from "remark-gfm";
import remarkRehype from "remark-rehype";
import rehypeSlug from "rehype-slug";
import rehypeShiki from "@shikijs/rehype";
import rehypeStringify from "rehype-stringify";
import { visit, SKIP } from "unist-util-visit";
import type { Root, Text, PhrasingContent } from "mdast";
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

const wikiRe = /\[\[([^\[\]\n]+?)\]\]/g;

// Splits `[[key|label]]` out of text nodes. Code spans and blocks are separate
// node types, so a wikilink quoted as code stays code.
function remarkWikilinks() {
  return (tree: Root) => {
    visit(tree, "html", (node: any, i, parent: any) => {
      parent.children[i!] = { type: "text", value: node.value };
    });
    visit(tree, "text", (node: Text, i, parent: any) => {
      if (!node.value.includes("[[")) return;
      const out: PhrasingContent[] = [];
      let last = 0;
      for (const m of node.value.matchAll(wikiRe)) {
        if (m.index! > last) out.push({ type: "text", value: node.value.slice(last, m.index) });
        const ref = resolve(m[1]);
        out.push(
          ref.resolved
            ? {
                type: "link",
                url: `/a/${ref.key}`,
                children: [{ type: "text", value: ref.label }],
                data: { hProperties: { className: ["wl", `s-${ref.status ?? "none"}`], "data-type": ref.type } },
              }
            : ({
                type: "text",
                value: ref.label,
                data: { hName: "span", hProperties: { className: ["wl-missing"], title: "unresolved link" } },
              } as any),
        );
        last = m.index! + m[0].length;
      }
      if (last < node.value.length) out.push({ type: "text", value: node.value.slice(last) });
      parent.children.splice(i!, 1, ...out);
      return [SKIP, i! + out.length];
    });
  };
}

export type TocEntry = { depth: number; text: string; id: string };

const toText = (n: any): string => (n.value ?? "") + (n.children ?? []).map(toText).join("");

const processor = unified()
  .use(remarkParse)
  .use(remarkGfm)
  .use(remarkWikilinks)
  .use(remarkRehype)
  .use(rehypeSlug)
  .use(rehypeShiki, { theme: "github-dark-default", defaultLanguage: "text", fallbackLanguage: "text" })
  .use(rehypeStringify);

export async function render(md: string): Promise<{ html: string; toc: TocEntry[] }> {
  const hast: any = await processor.run(processor.parse(md));
  const toc: TocEntry[] = [];
  visit(hast, "element", (el: any) => {
    if (el.tagName === "h2" || el.tagName === "h3")
      toc.push({ depth: Number(el.tagName[1]), text: toText(el), id: el.properties.id });
  });
  return { html: processor.stringify(hast), toc };
}
