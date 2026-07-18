import recommended from "stylelint-config-recommended";

export default {
  ...recommended,
  referenceFiles: ["src/styles/tokens.css"],
  reportNeedlessDisables: true,
  rules: {
    ...recommended.rules,
    "block-no-empty": null,
    "custom-property-pattern": "^ad-[a-z0-9]+(?:-[a-z0-9]+)*$",
    "declaration-block-no-duplicate-custom-properties": true,
    "declaration-block-no-duplicate-properties": true,
    "declaration-no-important": true,
    "layer-name-pattern": "^ad-(reset|tokens|base|components|features|integrations|skins)$",
    "no-descending-specificity": null,
    "no-duplicate-at-import-rules": true,
    "no-duplicate-selectors": true,
    "no-unknown-custom-properties": true,
    "rule-nesting-at-rule-required-list": ["layer"],
    "selector-max-id": 0,
    "selector-max-specificity": "0,5,2"
  }
};
