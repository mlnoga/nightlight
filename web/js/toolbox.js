
var toolbox = {
  "kind": "categoryToolbox",
  "contents": [
    {
      "kind": "category",
      "name": "File",
      "categorystyle": "file_category",
      "contents": [
        // {
        //   "kind": "block",
        //   "type": "nl_file_sequence"
        // },
        {
          "kind": "block",
          "type": "nl_file_load"
        },
        {
          "kind": "block",
          "type": "nl_file_loadMany"
        },
        {
          "kind": "block",
          "type": "nl_file_save"
        },
      ]
    },
    {
      "kind": "category",
      "name": "Preprocess",
      "categorystyle": "pre_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_pre_calibrate"
        },
        {
          "kind": "block",
          "type": "nl_pre_badPixel"
        },
        {
          "kind": "block",
          "type": "nl_pre_debayer"
        },
        {
          "kind": "block",
          "type": "nl_pre_debandVert"
        },
        {
          "kind": "block",
          "type": "nl_pre_debandHoriz"
        },
        {
          "kind": "block",
          "type": "nl_pre_scaleOffset"
        },
        {
          "kind": "block",
          "type": "nl_pre_bin"
        },
        {
          "kind": "block",
          "type": "nl_pre_backExtract"
        },
        {
          "kind": "block",
          "type": "nl_pre_starDetect"
        }
      ]
    },
    {
      "kind": "category",
      "name": "Reference",
      "categorystyle": "ref_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_ref_selectReference"
        }
      ]
    },
    {
      "kind": "category",
      "name": "Postprocessing",
      "categorystyle": "post_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_post_matchHistogram"
        },
        {
          "kind": "block",
          "type": "nl_post_align"
        }
      ]
    },
    {
      "kind": "category",
      "name": "Stack",
      "categorystyle": "stack_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_stack_stack"
        },
        {
          "kind": "block",
          "type": "nl_stack_stackBatches"
        }
      ]
    },
    {
      "kind": "category",
      "name": "Stretch",
      "categorystyle": "stretch_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_stretch_normRange"
        },
        {
          "kind": "block",
          "type": "nl_stretch_stretch"
        },
        {
          "kind": "block",
          "type": "nl_stretch_midtones"
        },
        {
          "kind": "block",
          "type": "nl_stretch_gamma"
        },
        {
          "kind": "block",
          "type": "nl_stretch_gammaPP"
        },
        {
          "kind": "block",
          "type": "nl_stretch_scaleBlack"
        },
        {
          "kind": "block",
          "type": "nl_stretch_unsharpMask"
        },
      ]
    },
    {
      "kind": "category",
      "name": "RGB",
      "categorystyle": "rgb_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_rgb_rgbCombine"
        },
        {
          "kind": "block",
          "type": "nl_rgb_rgbBalance"
        },
        {
          "kind": "block",
          "type": "nl_rgb_rgbToHSLuv"
        },
        {
          "kind": "block",
          "type": "nl_rgb_hsluvToRGB"
        }
      ]
    },
    {
      "kind": "category",
      "name": "HSL",
      "categorystyle": "hsl_category",
      "contents": [
        {
          "kind": "block",
          "type": "nl_hsl_hslApplyLum"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslScaleOffsetChannel"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslNeutralizeBackground"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslSaturationGamma"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslSelectiveSaturation"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslRotateHue"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslSCNR"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslMidtones"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslGamma"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslGammaPP"
        },
        {
          "kind": "block",
          "type": "nl_hsl_hslScaleBlack"
        },
      ]
    },
  ]
};
