import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import postcss from "postcss";
import selectorParser from "postcss-selector-parser";
import valueParser from "postcss-value-parser";
import ts from "typescript";

const LAYERS = ["ad-reset", "ad-tokens", "ad-base", "ad-components", "ad-features", "ad-integrations", "ad-skins"];
const EXCEPTION_RULES = new Set(["inline-style", "raw-color", "raw-font", "raw-shadow", "raw-radius", "raw-spacing"]);
const DATA_ATTRIBUTES = ["data-ui", "data-slot", "data-state", "data-variant"];

function walk(dir, suffixes) {
  if (!fs.existsSync(dir)) return [];
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return walk(full, suffixes);
    return suffixes.some((suffix) => entry.name.endsWith(suffix)) ? [full] : [];
  }).sort();
}

function relative(root, file) {
  return path.relative(root, file).split(path.sep).join("/");
}

function expectedLayers(file) {
  if (file === "src/presentation/contract-fixture.css") return new Set(["ad-features", "ad-skins"]);
  if (file === "src/styles/foundation.css") return new Set(["ad-reset"]);
  if (file === "src/styles/tokens.css") return new Set(["ad-tokens"]);
  if (file === "src/styles/base.css") return new Set(["ad-base"]);
  if (file.startsWith("src/styles/components/")) return new Set(["ad-components"]);
  if (file.startsWith("src/styles/features/")) return new Set(["ad-features"]);
  if (file === "src/styles/integrations.css") return new Set(["ad-integrations"]);
  return new Set();
}

function layerOf(node) {
  for (let current = node.parent; current; current = current.parent) {
    if (current.type === "atrule" && current.name === "layer") return current.params.trim();
  }
  return null;
}

function stringValuesFromType(type) {
  if (type.flags & (ts.TypeFlags.Undefined | ts.TypeFlags.Null | ts.TypeFlags.Never)) return new Set();
  if (type.flags & ts.TypeFlags.StringLiteral) return new Set([type.value]);
  if (type.isUnion()) {
    const values = new Set();
    for (const member of type.types) {
      const memberValues = stringValuesFromType(member);
      if (memberValues == null) return null;
      for (const value of memberValues) values.add(value);
    }
    return values;
  }
  return null;
}

function expressionValues(expression, checker, depth = 0) {
  if (!expression || depth > 12) return null;
  if (ts.isStringLiteralLike(expression)) return new Set([expression.text]);
  if (ts.isParenthesizedExpression(expression) || ts.isAsExpression(expression) || ts.isSatisfiesExpression(expression) || ts.isNonNullExpression(expression)) {
    return expressionValues(expression.expression, checker, depth + 1);
  }
  if (ts.isConditionalExpression(expression)) {
    const yes = expressionValues(expression.whenTrue, checker, depth + 1);
    const no = expressionValues(expression.whenFalse, checker, depth + 1);
    if (yes == null || no == null) return null;
    return new Set([...yes, ...no]);
  }
  if (ts.isBinaryExpression(expression) && expression.operatorToken.kind === ts.SyntaxKind.PlusToken) {
    const left = expressionValues(expression.left, checker, depth + 1);
    const right = expressionValues(expression.right, checker, depth + 1);
    if (left == null || right == null) return null;
    return new Set([...left].flatMap((a) => [...right].map((b) => `${a}${b}`)));
  }
  if (ts.isTemplateExpression(expression)) {
    let values = new Set([expression.head.text]);
    for (const span of expression.templateSpans) {
      const spanValues = expressionValues(span.expression, checker, depth + 1) ?? stringValuesFromType(checker.getTypeAtLocation(span.expression));
      if (spanValues == null) return null;
      values = new Set([...values].flatMap((prefix) => [...spanValues].map((value) => `${prefix}${value}${span.literal.text}`)));
    }
    return values;
  }
  return stringValuesFromType(checker.getTypeAtLocation(expression));
}

function attributeExpression(attribute) {
  if (!attribute.initializer) return null;
  if (ts.isStringLiteral(attribute.initializer)) return attribute.initializer;
  if (ts.isJsxExpression(attribute.initializer)) return attribute.initializer.expression ?? null;
  return null;
}

function attributeValues(attribute, checker) {
  if (!attribute.initializer) return new Set([""]);
  if (ts.isStringLiteral(attribute.initializer)) return new Set([attribute.initializer.text]);
  return expressionValues(attribute.initializer.expression, checker);
}

function jsxAttribute(opening, name) {
  return opening.attributes.properties.find((property) => ts.isJsxAttribute(property) && property.name.text === name);
}

function jsxTagName(opening) {
  return opening.tagName.getText();
}

function classValues(attribute, checker) {
  const expression = attributeExpression(attribute);
  const direct = attributeValues(attribute, checker);
  if (direct != null) return direct;
  if (!expression) return new Set();
  const values = new Set();
  const findArrays = (node) => {
    if (ts.isArrayLiteralExpression(node)) {
      for (const element of node.elements) {
        const itemValues = expressionValues(element, checker);
        if (itemValues) for (const value of itemValues) values.add(value);
      }
    }
    ts.forEachChild(node, findArrays);
  };
  findArrays(expression);
  return values;
}

function rawInlineLiteral(styleAttribute) {
  const expression = attributeExpression(styleAttribute);
  if (!expression || !ts.isObjectLiteralExpression(expression)) return false;
  for (const property of expression.properties) {
    if (!ts.isPropertyAssignment(property)) continue;
    const value = property.initializer;
    if (ts.isNumericLiteral(value)) return true;
    if (ts.isStringLiteral(value) && !/^var\(--ad-[a-z0-9-]+\)$/i.test(value.text)) return true;
    if (ts.isConditionalExpression(value)) {
      for (const branch of [value.whenTrue, value.whenFalse]) {
        if (ts.isNumericLiteral(branch)) return true;
        if (ts.isStringLiteral(branch) && !/^var\(--ad-[a-z0-9-]+\)$/i.test(branch.text)) return true;
      }
    }
  }
  return false;
}

function usageRecord(contract) {
  return new Map(Object.entries(contract.components).map(([name]) => [name, {
    component: false,
    slots: new Set(),
    states: new Set(),
    variants: new Set(),
  }]));
}

function ownersForValue(contract, kind, value) {
  const key = kind === "data-slot" ? "slots" : kind === "data-state" ? "states" : "variants";
  return Object.entries(contract.components).filter(([, item]) => item[key].includes(value)).map(([name]) => name);
}

function markUsage(usage, owners, kind, value) {
  const key = kind === "data-slot" ? "slots" : kind === "data-state" ? "states" : "variants";
  for (const owner of owners) usage.get(owner)?.[key].add(value);
}

function programFor(root, files) {
  const configPath = path.join(root, "tsconfig.app.json");
  let options = { target: ts.ScriptTarget.ESNext, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX };
  let names = files;
  if (fs.existsSync(configPath)) {
    const config = ts.readConfigFile(configPath, ts.sys.readFile);
    const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, root);
    options = parsed.options;
    names = [...new Set([...parsed.fileNames, ...files])];
  }
  return ts.createProgram({ rootNames: names, options });
}

export function auditPresentation(root) {
  const diagnostics = [];
  const rawDiagnostics = [];
  const src = path.join(root, "src");
  const cssFiles = walk(src, [".css"]);
  const codeFiles = walk(src, [".tsx", ".ts"]);
  const contractPath = path.join(src, "presentation", "contract.json");
  const exceptionsPath = path.join(root, "presentation-exceptions.json");
  const contract = JSON.parse(fs.readFileSync(contractPath, "utf8"));
  const exceptions = JSON.parse(fs.readFileSync(exceptionsPath, "utf8"));
  const usage = usageRecord(contract);
  const selectorClasses = new Set();
  const tokenDefinitions = new Map();
  const tokenReferences = new Map();
  const publicTokens = new Set(contract.tokens);
  const allSlots = new Set(Object.values(contract.components).flatMap((item) => item.slots));
  const decorative = new Set(contract.decorative_slots);

  const add = (file, rule, message) => diagnostics.push({ file, rule, message });
  const addRaw = (file, rule, message) => rawDiagnostics.push({ file, rule, message });

  if (contract.version !== 1) add("src/presentation/contract.json", "manifest", "version must be 1");
  if (!Array.isArray(contract.tokens) || !contract.components || !Array.isArray(contract.decorative_slots)) add("src/presentation/contract.json", "manifest", "invalid contract shape");
  if (new Set(contract.tokens).size !== contract.tokens.length) add("src/presentation/contract.json", "manifest", "duplicate public token");
  for (const [name, item] of Object.entries(contract.components)) {
    if (!/^[a-z][a-z0-9-]*$/.test(name)) add("src/presentation/contract.json", "manifest", `invalid data-ui name ${name}`);
    for (const key of ["slots", "states", "variants"]) {
      if (!Array.isArray(item[key]) || new Set(item[key]).size !== item[key].length) add("src/presentation/contract.json", "manifest", `${name}.${key} must be a unique array`);
    }
  }

  for (const absolute of cssFiles) {
    const file = relative(root, absolute);
    const source = fs.readFileSync(absolute, "utf8");
    let cssRoot;
    try {
      cssRoot = postcss.parse(source, { from: file });
    } catch (error) {
      add(file, "css-parse", error.reason ?? error.message);
      continue;
    }
    cssRoot.walkComments((comment) => {
      if (/stylelint-disable/.test(comment.text)) add(file, "inline-disable", "inline Stylelint disables are prohibited");
    });
    cssRoot.walkRules((rule) => {
      const layer = layerOf(rule);
      const expected = expectedLayers(file);
      if (file !== "src/styles/index.css" && (!layer || !expected.has(layer))) add(file, "cascade-layer", `selector ${rule.selector} is outside its declared layer`);
      if (layer === "ad-skins" && file !== "src/presentation/contract-fixture.css") add(file, "production-skin", "production ad-skins layer must be empty");
      try {
        selectorParser((selectors) => {
          selectors.each((selector) => {
            const hooks = { "data-ui": [], "data-slot": [], "data-state": [], "data-variant": [] };
            selector.walkClasses((node) => selectorClasses.add(node.value));
            selector.walkAttributes((node) => {
              if (node.attribute === "data-skin") add(file, "skin-state", "core CSS must not depend on data-skin");
              if (DATA_ATTRIBUTES.includes(node.attribute) && node.operator === "=" && node.value) hooks[node.attribute].push(node.value.replace(/^['"]|['"]$/g, ""));
            });
            const owners = hooks["data-ui"];
            for (const name of owners) {
              if (!contract.components[name]) add(file, "hook", `undocumented data-ui ${name}`);
              else usage.get(name).component = true;
            }
            for (const kind of ["data-slot", "data-state", "data-variant"]) {
              for (const value of hooks[kind]) {
                const inferred = owners.length ? owners : ownersForValue(contract, kind, value);
                if (inferred.length === 0) add(file, "hook", `undocumented ${kind} ${value}`);
                for (const owner of inferred) {
                  const item = contract.components[owner];
                  const key = kind === "data-slot" ? "slots" : kind === "data-state" ? "states" : "variants";
                  if (!item?.[key].includes(value)) add(file, "hook", `${owner} does not declare ${kind} ${value}`);
                }
                markUsage(usage, inferred, kind, value);
              }
            }
          });
        }).processSync(rule.selector);
      } catch (error) {
        add(file, "selector-parse", error.message);
      }
    });
    cssRoot.walkDecls((declaration) => {
      const layer = layerOf(declaration);
      if (declaration.prop.startsWith("--ad-")) {
        const list = tokenDefinitions.get(declaration.prop) ?? [];
        list.push({ file, layer });
        tokenDefinitions.set(declaration.prop, list);
        if (publicTokens.has(declaration.prop) && file !== "src/styles/tokens.css" && layer !== "ad-skins") add(file, "skin-layer", `${declaration.prop} override is outside ad-skins`);
      }
      const parsed = valueParser(declaration.value);
      parsed.walk((node) => {
        if (node.type === "function" && node.value === "var" && node.nodes[0]?.value?.startsWith("--ad-")) {
          tokenReferences.set(node.nodes[0].value, (tokenReferences.get(node.nodes[0].value) ?? 0) + 1);
        }
      });
      const rawAllowed = ["src/styles/tokens.css", "src/styles/foundation.css", "src/presentation/contract-fixture.css"].includes(file);
      if (rawAllowed) return;
      if (/(?:#[0-9a-f]{3,8}\b|\b(?:rgb|hsl)a?\()/i.test(declaration.value)) addRaw(file, "raw-color", `${declaration.prop} uses a raw color`);
      if (declaration.prop === "font-family" && !/^var\(/.test(declaration.value)) addRaw(file, "raw-font", "font-family must use a token");
      if (/(?:box|text)-shadow/.test(declaration.prop) && !/^(?:none|var\()/.test(declaration.value)) addRaw(file, "raw-shadow", `${declaration.prop} must use a token`);
      if (/border(?:-[a-z]+){0,2}-radius/.test(declaration.prop) && !/^(?:0|var\()/.test(declaration.value)) addRaw(file, "raw-radius", `${declaration.prop} must use a token`);
      if (/^(?:gap|row-gap|column-gap|margin(?:-[a-z]+)?|padding(?:-[a-z]+)?)$/.test(declaration.prop) && /\b\d+(?:\.\d+)?(?:px|r?em)\b/.test(declaration.value)) addRaw(file, "raw-spacing", `${declaration.prop} must use spacing tokens`);
    });
  }

  const program = programFor(root, codeFiles);
  const checker = program.getTypeChecker();
  const importedStyles = [];
  let visualMatrixStaticImport = false;

  for (const absolute of codeFiles) {
    const file = relative(root, absolute);
    const sourceFile = program.getSourceFile(absolute) ?? ts.createSourceFile(absolute, fs.readFileSync(absolute, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

    const visit = (node, owners = []) => {
      if (ts.isImportDeclaration(node) && ts.isStringLiteral(node.moduleSpecifier)) {
        const imported = node.moduleSpecifier.text;
        if (imported.endsWith(".css")) importedStyles.push({ file, imported });
        if (imported.includes("VisualMatrix") && !file.endsWith(".test.tsx")) visualMatrixStaticImport = true;
      }
      if (ts.isJsxElement(node)) {
        const nextOwners = inspectOpening(node.openingElement, owners, file);
        for (const child of node.children) visit(child, nextOwners);
        return;
      }
      if (ts.isJsxSelfClosingElement(node)) {
        inspectOpening(node, owners, file);
        return;
      }
      ts.forEachChild(node, (child) => visit(child, owners));
    };

    const inspectOpening = (opening, inheritedOwners, sourceName) => {
      const uiAttribute = jsxAttribute(opening, "data-ui");
      let owners = inheritedOwners;
      if (uiAttribute) {
        const values = attributeValues(uiAttribute, checker);
        if (values == null || values.size === 0) {
          add(sourceName, "hook", "data-ui must resolve to a finite string-literal union");
          owners = [];
        } else {
          owners = [...values];
          for (const name of owners) {
            if (!contract.components[name]) add(sourceName, "hook", `undocumented data-ui ${name}`);
            else usage.get(name).component = true;
          }
        }
      }

      const classAttribute = jsxAttribute(opening, "className");
      if (classAttribute) {
        const values = classValues(classAttribute, checker);
        for (const value of values ?? []) {
          for (const name of value.split(/\s+/).filter(Boolean)) {
            if (/^[a-z_][\w-]*$/i.test(name) && !selectorClasses.has(name)) add(sourceName, "class-selector", `literal class \"${name}\" has no CSS selector`);
          }
        }
      }

      for (const kind of ["data-slot", "data-state", "data-variant"]) {
        const attribute = jsxAttribute(opening, kind);
        if (!attribute) continue;
        const values = attributeValues(attribute, checker);
        if (values == null) {
          add(sourceName, "hook", `${kind} must resolve to a finite string-literal union`);
          continue;
        }
        for (const value of values) {
          if (!value) continue;
          const inferred = owners.length ? owners : ownersForValue(contract, kind, value);
          if (inferred.length === 0) add(sourceName, "hook", `undocumented ${kind} ${value}`);
          for (const owner of inferred) {
            const item = contract.components[owner];
            const key = kind === "data-slot" ? "slots" : kind === "data-state" ? "states" : "variants";
            if (!item?.[key].includes(value)) add(sourceName, "hook", `${owner} does not declare ${kind} ${value}`);
          }
          markUsage(usage, inferred, kind, value);
        }
      }

      const styleAttribute = jsxAttribute(opening, "style");
      if (styleAttribute && /^[a-z]/.test(jsxTagName(opening))) {
        addRaw(sourceName, "inline-style", "native inline style requires an exact exception");
        if (rawInlineLiteral(styleAttribute)) add(sourceName, "inline-style-literal", "inline style contains a static visual literal");
      }
      return owners;
    };

    visit(sourceFile, []);
  }

  for (const [token, count] of tokenReferences) {
    if (!tokenDefinitions.has(token)) add("src/styles", "token", `undefined token ${token}`);
    void count;
  }
  for (const token of contract.tokens) {
    const definitions = (tokenDefinitions.get(token) ?? []).filter(({ file }) => file !== "src/presentation/contract-fixture.css");
    if (definitions.length !== 1) add("src/styles/tokens.css", "token", `${token} must have exactly one core definition (found ${definitions.length})`);
    if (!tokenReferences.has(token)) add("src/styles/tokens.css", "token", `${token} is public but unused by core CSS`);
  }

  for (const [name, item] of Object.entries(contract.components)) {
    const seen = usage.get(name);
    if (!seen.component) add("src/presentation/contract.json", "manifest-stale", `data-ui ${name} has no implementation`);
    for (const value of item.slots) if (!seen.slots.has(value)) add("src/presentation/contract.json", "manifest-stale", `${name} slot ${value} has no implementation`);
    for (const value of item.states) if (!seen.states.has(value)) add("src/presentation/contract.json", "manifest-stale", `${name} state ${value} has no implementation`);
    for (const value of item.variants) if (!seen.variants.has(value)) add("src/presentation/contract.json", "manifest-stale", `${name} variant ${value} has no implementation`);
  }
  for (const slot of decorative) if (!selectorClasses.has(slot)) add("src/presentation/contract.json", "manifest-stale", `decorative slot ${slot} has no implementation`);

  const indexPath = path.join(src, "styles", "index.css");
  const index = fs.readFileSync(indexPath, "utf8");
  if (!index.includes(`@layer ${LAYERS.join(", ")};`)) add("src/styles/index.css", "cascade", "missing fixed cascade declaration");
  if (!index.includes("@layer ad-skins {}")) add("src/styles/index.css", "skin-layer", "production ad-skins layer must be explicitly empty");
  if (/contract-fixture\.css/.test(index)) add("src/styles/index.css", "fixture-production", "development fixture is imported by production CSS");
  const permittedImports = new Set(["./styles/index.css", "@xterm/xterm/css/xterm.css", "./contract-fixture.css"]);
  for (const { file, imported } of importedStyles) if (!permittedImports.has(imported)) add(file, "css-import", `unsupported stylesheet import ${imported}`);
  if (visualMatrixStaticImport) add("src/routes.tsx", "fixture-production", "VisualMatrix must be dynamically imported behind the development gate");
  const routes = fs.readFileSync(path.join(src, "routes.tsx"), "utf8");
  if (!routes.includes("import.meta.env.DEV") || !routes.includes('import("./presentation/VisualMatrix")')) add("src/routes.tsx", "fixture-gate", "visual matrix must remain development-only");

  const packageJson = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
  if (packageJson.scripts?.pretest !== "npm run check:styles") add("package.json", "wiring", "pretest must run check:styles");
  if (packageJson.scripts?.prebuild !== "npm run check:styles") add("package.json", "wiring", "prebuild must run check:styles");
  if (!packageJson.scripts?.["check:styles"]?.includes("--max-warnings 0")) add("package.json", "wiring", "check:styles must reject Stylelint warnings");

  const matchedExceptions = new Set();
  const exceptionKeys = new Set();
  for (const entry of exceptions) {
    const key = `${entry.file}:${entry.rule}`;
    if (exceptionKeys.has(key)) add("presentation-exceptions.json", "exception", `duplicate exception ${key}`);
    exceptionKeys.add(key);
    if (!EXCEPTION_RULES.has(entry.rule)) add("presentation-exceptions.json", "exception", `unknown exception rule ${entry.rule}`);
    if (!entry.file || path.isAbsolute(entry.file) || entry.file.includes("..") || !fs.existsSync(path.join(root, entry.file))) add("presentation-exceptions.json", "exception", `missing or unsafe exception file ${entry.file}`);
    if (!entry.reason || entry.reason.trim().length < 12) add("presentation-exceptions.json", "exception", `${key} needs a specific reason`);
  }
  for (const diagnostic of rawDiagnostics) {
    const key = `${diagnostic.file}:${diagnostic.rule}`;
    if (exceptionKeys.has(key)) matchedExceptions.add(key);
    else diagnostics.push(diagnostic);
  }
  for (const key of exceptionKeys) if (!matchedExceptions.has(key)) add("presentation-exceptions.json", "exception", `stale exception ${key}`);

  return diagnostics
    .map(({ file, rule, message }) => `${file} [${rule}] ${message}`)
    .sort();
}

const invokedPath = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (invokedPath === fileURLToPath(import.meta.url)) {
  const failures = auditPresentation(process.cwd());
  if (failures.length) {
    console.error(failures.map((failure) => `- ${failure}`).join("\n"));
    process.exitCode = 1;
  } else {
    console.log("Presentation contract check passed.");
  }
}
