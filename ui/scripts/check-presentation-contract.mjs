import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import postcss from "postcss";
import selectorParser from "postcss-selector-parser";
import valueParser from "postcss-value-parser";
import ts from "typescript";

const LAYERS = ["ad-reset", "ad-tokens", "ad-base", "ad-components", "ad-features", "ad-integrations", "ad-skins"];
const EXCEPTION_RULES = new Set(["inline-style", "raw-color", "raw-font", "raw-shadow", "raw-radius", "raw-spacing", "native-dialog", "renderer-markup"]);
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
  if (file.startsWith("src/styles/skins/")) return new Set(["ad-skins"]);
  if (file === "src/styles/integrations.css") return new Set(["ad-integrations"]);
  return new Set();
}

function skinForFile(file) {
  return /^src\/styles\/skins\/([a-z][a-z0-9-]*)\.css$/.exec(file)?.[1] ?? null;
}

function unwrappedExpression(expression) {
  let current = expression;
  while (current && (ts.isParenthesizedExpression(current) || ts.isAsExpression(current) || ts.isSatisfiesExpression(current) || ts.isNonNullExpression(current))) {
    current = current.expression;
  }
  return current;
}

function literalArray(expression) {
  const array = unwrappedExpression(expression);
  if (!array || !ts.isArrayLiteralExpression(array)) return null;
  const values = [];
  for (const element of array.elements) {
    if (!ts.isStringLiteralLike(element)) return null;
    values.push(element.text);
  }
  return values;
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

function nativeDialogReference(expression, aliases = new Map()) {
  if (!expression) return null;
  if (ts.isParenthesizedExpression(expression) || ts.isAsExpression(expression) || ts.isSatisfiesExpression(expression) || ts.isNonNullExpression(expression)) {
    return nativeDialogReference(expression.expression, aliases);
  }
  if (ts.isIdentifier(expression)) {
    if (["prompt", "confirm"].includes(expression.text)) return expression.text;
    return aliases.get(expression.text) ?? null;
  }
  const global = (node) => ts.isIdentifier(node) && ["window", "globalThis"].includes(node.text);
  if (ts.isPropertyAccessExpression(expression) && global(expression.expression) && ["prompt", "confirm"].includes(expression.name.text)) return expression.name.text;
  if (ts.isElementAccessExpression(expression) && global(expression.expression) && ts.isStringLiteralLike(expression.argumentExpression) && ["prompt", "confirm"].includes(expression.argumentExpression.text)) return expression.argumentExpression.text;
  return null;
}

function nativeDialogAliases(sourceFile) {
  const aliases = new Map();
  const global = (node) => ts.isIdentifier(node) && ["window", "globalThis"].includes(node.text);
  const visit = (node) => {
    if (ts.isVariableDeclaration(node) && node.initializer) {
      const nativeDialog = nativeDialogReference(node.initializer, aliases);
      if (nativeDialog && ts.isIdentifier(node.name)) aliases.set(node.name.text, nativeDialog);
      if (ts.isObjectBindingPattern(node.name) && global(node.initializer)) {
        for (const element of node.name.elements) {
          const property = element.propertyName ?? element.name;
          if (ts.isIdentifier(property) && ts.isIdentifier(element.name) && ["prompt", "confirm"].includes(property.text)) aliases.set(element.name.text, property.text);
        }
      }
    }
    if (ts.isBinaryExpression(node) && node.operatorToken.kind === ts.SyntaxKind.EqualsToken && ts.isIdentifier(node.left)) {
      const nativeDialog = nativeDialogReference(node.right, aliases);
      if (nativeDialog) aliases.set(node.left.text, nativeDialog);
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return aliases;
}

function nativeDialogCall(node, aliases) {
  return ts.isCallExpression(node) ? nativeDialogReference(node.expression, aliases) : null;
}

function nativeDialogExempt(file) {
  return /\.test\.[jt]sx?$/.test(file) || file === "src/presentation/VisualMatrix.tsx";
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
  const declaredSkins = new Set(contract.skins ?? []);
  const allSlots = new Set(Object.values(contract.components).flatMap((item) => item.slots));
  const decorative = new Set(contract.decorative_slots);
  const activeSkinSelectors = new Set();
  const previewSkinSelectors = new Set();
  const cssImports = [];

  const add = (file, rule, message) => diagnostics.push({ file, rule, message });
  const addRaw = (file, rule, message) => rawDiagnostics.push({ file, rule, message });

  if (contract.version !== 2) add("src/presentation/contract.json", "manifest", "version must be 2");
  if (!Array.isArray(contract.skins) || !Array.isArray(contract.tokens) || !contract.components || !Array.isArray(contract.decorative_slots)) add("src/presentation/contract.json", "manifest", "invalid contract shape");
  if (new Set(contract.skins ?? []).size !== (contract.skins ?? []).length) add("src/presentation/contract.json", "manifest", "duplicate built-in skin id");
  for (const skin of contract.skins ?? []) {
    if (!/^[a-z][a-z0-9-]*$/.test(skin) || skin === "core") add("src/presentation/contract.json", "manifest", `invalid built-in skin id ${skin}`);
  }
  if (new Set(contract.tokens).size !== contract.tokens.length) add("src/presentation/contract.json", "manifest", "duplicate public token");
  for (const [name, item] of Object.entries(contract.components)) {
    if (!/^[a-z][a-z0-9-]*$/.test(name)) add("src/presentation/contract.json", "manifest", `invalid data-ui name ${name}`);
    for (const key of ["slots", "states", "variants"]) {
      if (!Array.isArray(item[key]) || new Set(item[key]).size !== item[key].length) add("src/presentation/contract.json", "manifest", `${name}.${key} must be a unique array`);
    }
  }
  if (declaredSkins.size && !contract.components["config-editor"]?.variants?.includes("appearance")) {
    add("src/presentation/contract.json", "manifest", "built-in skins require the config-editor appearance variant");
  }

  for (const absolute of cssFiles) {
    const file = relative(root, absolute);
    const fileSkin = skinForFile(file);
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
    cssRoot.walkAtRules("import", (atRule) => {
      const imported = atRule.params.trim().replace(/^url\(\s*|\s*\)$/g, "").replace(/^['\"]|['\"]$/g, "");
      cssImports.push({ file, imported });
      if (/^(?:https?:)?\/\//i.test(imported)) add(file, "network-asset", `network stylesheet import ${imported} is prohibited`);
    });
    if (fileSkin && !declaredSkins.has(fileSkin)) add(file, "skin-file", `skin stylesheet ${fileSkin} is not declared by the manifest`);
    cssRoot.walkRules((rule) => {
      const layer = layerOf(rule);
      const expected = expectedLayers(file);
      if (file !== "src/styles/index.css" && (!layer || !expected.has(layer))) add(file, "cascade-layer", `selector ${rule.selector} is outside its declared layer`);
      if (layer === "ad-skins" && file !== "src/presentation/contract-fixture.css" && !fileSkin) add(file, "skin-file", "production ad-skins rules belong only in a declared skin stylesheet");
      try {
        selectorParser((selectors) => {
          selectors.each((selector) => {
            const hooks = { "data-ui": [], "data-slot": [], "data-state": [], "data-variant": [] };
            const skinValues = [];
            const previewValues = [];
            selector.walkClasses((node) => selectorClasses.add(node.value));
            selector.walkAttributes((node) => {
              const value = node.operator === "=" && node.value ? node.value.replace(/^['"]|['"]$/g, "") : null;
              if (node.attribute === "data-skin") skinValues.push(value);
              if (node.attribute === "data-preview-skin") previewValues.push(value);
              if (DATA_ATTRIBUTES.includes(node.attribute) && node.operator === "=" && node.value) hooks[node.attribute].push(node.value.replace(/^['"]|['"]$/g, ""));
            });
            if (fileSkin) {
              const selectorText = selector.toString().trim();
              if (skinValues.length) {
                const startsAtRoot = selector.nodes[0]?.type === "pseudo" && selector.nodes[0].value === ":root";
                if (!startsAtRoot || skinValues.some((value) => value !== fileSkin)) {
                  add(file, "skin-scope", `selector ${selectorText} must use :root[data-skin="${fileSkin}"]`);
                } else {
                  activeSkinSelectors.add(fileSkin);
                }
              } else if (previewValues.length) {
                const appearancePreview = previewValues.every((value) => value === fileSkin)
                  && hooks["data-ui"].includes("config-editor")
                  && hooks["data-variant"].includes("appearance");
                if (!appearancePreview) add(file, "skin-scope", `selector ${selectorText} must be scoped to the ${fileSkin} Settings appearance preview`);
                else previewSkinSelectors.add(fileSkin);
              } else if (selectorText !== ":root") {
                add(file, "skin-scope", `selector ${selectorText} is not scoped to ${fileSkin}`);
              }
            } else if (file !== "src/presentation/contract-fixture.css") {
              if (skinValues.length) add(file, "skin-state", "Core CSS must not depend on data-skin");
              for (const value of previewValues) {
                if (value !== "core") add(file, "skin-scope", `preview skin ${value ?? "<dynamic>"} belongs only in its declared skin stylesheet`);
              }
            }
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
      const privatePrefix = fileSkin ? `--ad-${fileSkin}-` : null;
      const parentSelector = declaration.parent?.type === "rule" ? declaration.parent.selector.trim() : "";
      if (fileSkin && parentSelector === ":root" && !declaration.prop.startsWith(privatePrefix)) {
        add(file, "skin-token", `bare :root may declare only private ${privatePrefix}* palette tokens`);
      }
      if (declaration.prop.startsWith("--ad-")) {
        const list = tokenDefinitions.get(declaration.prop) ?? [];
        list.push({ file, layer, selector: parentSelector });
        tokenDefinitions.set(declaration.prop, list);
        if (publicTokens.has(declaration.prop) && !["src/styles/tokens.css", "src/presentation/contract-fixture.css"].includes(file) && !(fileSkin && layer === "ad-skins")) add(file, "skin-layer", `${declaration.prop} override is outside a declared ad-skins stylesheet`);
        if (fileSkin && !publicTokens.has(declaration.prop) && !declaration.prop.startsWith(privatePrefix)) add(file, "skin-token", `${declaration.prop} is neither public nor private to ${fileSkin}`);
        if (fileSkin && declaration.prop.startsWith(privatePrefix) && parentSelector !== ":root") add(file, "skin-token", `${declaration.prop} must be declared once on :root`);
        for (const skin of declaredSkins) {
          if (declaration.prop.startsWith(`--ad-${skin}-`) && fileSkin !== skin) add(file, "skin-token", `${declaration.prop} may be declared only in ${skin}.css`);
        }
      }
      const parsed = valueParser(declaration.value);
      parsed.walk((node) => {
        if (node.type === "function" && node.value === "var" && node.nodes[0]?.value?.startsWith("--ad-")) {
          const token = node.nodes[0].value;
          const references = tokenReferences.get(token) ?? [];
          references.push({ file });
          tokenReferences.set(token, references);
          for (const skin of declaredSkins) {
            if (token.startsWith(`--ad-${skin}-`) && fileSkin !== skin) add(file, "skin-token", `${token} may be used only in ${skin}.css`);
          }
        }
        if (node.type === "function" && node.value.toLowerCase() === "url") {
          const target = valueParser.stringify(node.nodes).trim().replace(/^['"]|['"]$/g, "");
          if (/^(?:https?:)?\/\//i.test(target)) add(file, "network-asset", `network asset ${target} is prohibited`);
        }
      });
      const rawAllowed = ["src/styles/tokens.css", "src/styles/foundation.css", "src/presentation/contract-fixture.css"].includes(file)
        || Boolean(fileSkin && declaration.prop.startsWith(privatePrefix));
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
  const frontendSkinAllowlists = [];
  let appearanceVariantImplementation = false;
  let visualMatrixStaticImport = false;

  for (const absolute of codeFiles) {
    const file = relative(root, absolute);
    const sourceFile = program.getSourceFile(absolute) ?? ts.createSourceFile(absolute, fs.readFileSync(absolute, "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
    const nativeDialogAliasMap = nativeDialogAliases(sourceFile);

    const visit = (node, owners = []) => {
      const nativeDialog = nativeDialogCall(node, nativeDialogAliasMap);
      if (nativeDialog && !nativeDialogExempt(file)) addRaw(file, "native-dialog", `browser-native ${nativeDialog}() is prohibited`);
      if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.name.text === "BUILT_IN_SKINS") {
        const values = literalArray(node.initializer);
        if (values == null) add(file, "skin-allowlist", "BUILT_IN_SKINS must be a string-literal array");
        else frontendSkinAllowlists.push({ file, values });
      }
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
          if (kind === "data-variant" && value === "appearance" && owners.includes("config-editor")) appearanceVariantImplementation = true;
        }
      }

      for (const kind of ["data-skin", "data-preview-skin"]) {
        const attribute = jsxAttribute(opening, kind);
        if (!attribute) continue;
        const values = attributeValues(attribute, checker);
        if (values == null) {
          add(sourceName, "skin-state", `${kind} must resolve to a finite string-literal union`);
          continue;
        }
        for (const value of values) {
          const allowed = kind === "data-preview-skin" ? value === "core" || declaredSkins.has(value) : declaredSkins.has(value);
          if (!allowed) add(sourceName, "skin-state", `undocumented ${kind} ${value || "<empty>"}`);
        }
      }

      if (jsxAttribute(opening, "dangerouslySetInnerHTML")) {
        addRaw(sourceName, "renderer-markup", "renderer-produced markup requires an exact exception");
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

  for (const [token, references] of tokenReferences) {
    if (!tokenDefinitions.has(token)) add("src/styles", "token", `undefined token ${token}`);
    void references;
  }
  for (const token of contract.tokens) {
    const coreDefinitions = (tokenDefinitions.get(token) ?? []).filter(({ file, layer }) => file === "src/styles/tokens.css" && layer === "ad-tokens");
    const coreReferences = (tokenReferences.get(token) ?? []).filter(({ file }) => !skinForFile(file) && file !== "src/presentation/contract-fixture.css");
    if (coreDefinitions.length !== 1) add("src/styles/tokens.css", "token", `${token} must have exactly one Core definition (found ${coreDefinitions.length})`);
    if (!coreReferences.length) add("src/styles/tokens.css", "token", `${token} is public but unused by Core CSS`);
  }
  for (const skin of declaredSkins) {
    const prefix = `--ad-${skin}-`;
    for (const [token, definitions] of tokenDefinitions) {
      if (!token.startsWith(prefix)) continue;
      if (definitions.length !== 1) add(`src/styles/skins/${skin}.css`, "skin-token", `${token} must have exactly one private definition (found ${definitions.length})`);
      if (!tokenReferences.has(token)) add(`src/styles/skins/${skin}.css`, "skin-token", `${token} is private but unused`);
    }
  }

  if (declaredSkins.size) {
    if (!appearanceVariantImplementation) add("src/features/settings", "skin-hook", "config-editor appearance variant has no TSX implementation");
    if (frontendSkinAllowlists.length !== 1) add("src/features/appearance", "skin-allowlist", `expected exactly one BUILT_IN_SKINS declaration (found ${frontendSkinAllowlists.length})`);
    for (const { file, values } of frontendSkinAllowlists) {
      if (values.length !== declaredSkins.size || values.some((value) => !declaredSkins.has(value))) add(file, "skin-allowlist", "BUILT_IN_SKINS must exactly match contract.json skins");
    }
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
  if (/contract-fixture\.css/.test(index)) add("src/styles/index.css", "fixture-production", "development fixture is imported by production CSS");
  const indexImports = cssImports.filter(({ file }) => file === "src/styles/index.css").map(({ imported }) => imported);
  for (const skin of declaredSkins) {
    const stylesheet = `src/styles/skins/${skin}.css`;
    const imported = `./skins/${skin}.css`;
    if (!fs.existsSync(path.join(root, stylesheet))) add(stylesheet, "skin-file", `manifest skin ${skin} has no production stylesheet`);
    if (indexImports.filter((value) => value === imported).length !== 1) add("src/styles/index.css", "skin-import", `${imported} must be statically imported exactly once`);
    if (!activeSkinSelectors.has(skin)) add(stylesheet, "skin-scope", `${skin} has no active :root[data-skin] mapping`);
    if (!previewSkinSelectors.has(skin)) add(stylesheet, "skin-scope", `${skin} has no Settings appearance preview selector`);
  }
  for (const imported of indexImports) {
    const match = /^\.\/skins\/([a-z][a-z0-9-]*)\.css$/.exec(imported);
    if (match && !declaredSkins.has(match[1])) add("src/styles/index.css", "skin-import", `undeclared skin import ${imported}`);
  }
  const permittedImports = new Set(["./styles/index.css", "@xterm/xterm/css/xterm.css", "./contract-fixture.css"]);
  for (const { file, imported } of importedStyles) if (!permittedImports.has(imported)) add(file, "css-import", `unsupported stylesheet import ${imported}`);
  for (const { file, imported } of importedStyles) {
    if (imported === "./contract-fixture.css" && file !== "src/presentation/VisualMatrix.tsx") add(file, "fixture-production", "contract fixture may be imported only by VisualMatrix");
  }
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
