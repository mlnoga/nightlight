// Color theme
//

var theme = Blockly.Theme.defineTheme('nl_dark', {
  'base': Blockly.Themes.Classic,
  'componentStyles': {
    'workspaceBackgroundColour': '#050505',
    'toolboxBackgroundColour': '#101010',
    'toolboxForegroundColour': '#C00000',
    'flyoutBackgroundColour': '#202020',
    'flyoutForegroundColour': '#C00000',
    'flyoutOpacity': 1,
    'scrollbarColour': '#800000',
    'insertionMarkerColour': '#F0F0F0',
    'insertionMarkerOpacity': 0.3,
    'scrollbarOpacity': 0.4,
    'cursorColour': '#d00000',
    'blackBackground': '#000000',
  },
  'categoryStyles' : {
    "file_category": {
      "colour": "#661414",
    },
    "pre_category": {
      "colour": "#663C12",
    },
    "ref_category": {
      "colour": "#666613",
    },
    "post_category": {
      "colour": "#3D6614",
    },
    "stack_category": {
      "colour": "#136613",
    },
    "stretch_category": {
      "colour": "#13663C",
    },
    "rgb_category": {
      "colour": "#136666",
    },
    "hsl_category": {
      "colour": "#143D66",
    },
  },
  'blockStyles': {
     // based on https://www.rapidtables.com/web/color/color-picker.html
     // with hues 0, 30, 60, 90, 120 with sat 80%, values 40% - 25% - 10%
     "file_blocks": {
        "colourPrimary": "#661414",
        "colourSecondary":"#400D0D",
        "colourTertiary":"#1A0505"
     },
     "pre_blocks": {
        "colourPrimary": "#663C12",
        "colourSecondary":"#40260C",
        "colourTertiary":"#1A0F05"
     },
     "ref_blocks": {
        "colourPrimary": "#666613",
        "colourSecondary":"#40400C",
        "colourTertiary":"#1A1A05"
     },
     "post_blocks": {
        "colourPrimary": "#3D6614",
        "colourSecondary":"#26400C",
        "colourTertiary":"#0F1A05"
     },
     "stack_blocks": {
        "colourPrimary": "#136613",
        "colourSecondary":"#0C400C",
        "colourTertiary":"#051A05"
     },
     "stretch_blocks": {
        "colourPrimary": "#13663C",
        "colourSecondary":"#0C4026",
        "colourTertiary":"#051A0F"
     },
     "rgb_blocks": {
        "colourPrimary": "#136666",
        "colourSecondary":"#0C4040",
        "colourTertiary":"#051A1A"
     },
     "hsl_blocks": {
        "colourPrimary": "#143D66",
        "colourSecondary":"#0D2640",
        "colourTertiary":"#050F1A"
     },
  },
});

Blockly.HSV_SATURATION=0.45 // 0 (inclusive) to 1 (exclusive), defaulting to 0.45
Blockly.HSV_VALUE=0.3 // 0 (inclusive) to 1 (exclusive), defaulting to 0.65
