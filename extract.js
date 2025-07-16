// extract.js
import { unified } from "unified";
import remarkParse from "remark-parse";

const body = process.argv[2] || "";

const tree = unified().use(remarkParse).parse(body);

let typeBlock = "";
let purposeBlock = "";

let collecting = null;

for (const node of tree.children) {
  if (node.type === "heading") {
    const h = node.children[0].value.toLowerCase();
    if (h.includes("type of change")) collecting = "type";
    else if (h.includes("purpose")) collecting = "purpose";
    else collecting = null;
    continue;
  }

  if (!collecting) continue;

  const raw = body.slice(node.position.start.offset, node.position.end.offset);

  if (collecting === "type") typeBlock += raw + "\n";
  if (collecting === "purpose") purposeBlock += raw + "\n";
}

console.log(`::set-output name=type_block::${typeBlock.trim().replace(/\n/g, '%0A')}`);
console.log(`::set-output name=purpose_block::${purposeBlock.trim().replace(/\n/g, '%0A')}`);
