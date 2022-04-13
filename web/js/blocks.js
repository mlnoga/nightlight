Blockly.defineBlocksWithJsonArray([
  // File operators
  //

  {
    "type": "nl_file_load",
    "tooltip": "Load a FITS image from a file",
    "message0": "Load single image %1",
    "args0": [
      {
        "type": "field_input",
        "name": "fileName",
        "text": "image.fits",
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "file_blocks",
  },

  {
    "type": "nl_file_loadMany",
    "tooltip": "Load many FITS images from a filename pattern with wildcards * and ?",
    "message0": "Load many images %1",
    "args0": [
      {
        "type": "field_input",
        "name": "filePattern",
        "text": "*.fits",
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "file_blocks",
  },

  {
    "type": "nl_file_save",
    "tooltip": "Save image to FITS or JPEG based on extension, expanding %d to the image ID",
    "message0": "Save image to %1",
    "args0": [
      {
        "type": "field_input",
        "name": "filePattern",
        "text": "out%3d.fits",
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "file_blocks",
  },


  // Preprocessing operators
  //
  {
    "type": "nl_pre_calibrate",
    "tooltip": "Calibrate image with a (master) dark frame and a (master) flat frame",
    "message0": "Calibrate with dark %1 and with flat %2",
    "args0": [
      {
        "type": "field_input",
        "name": "dark",
        "text": "dark.fits",
      },
      {
        "type": "field_input",
        "name": "flat",
        "text": "flat.fits",
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_badPixel",
    "tooltip": "Cosmetic correction of pixels whose value is more than a given number of standard deviations, or sigmas, away from the local mean",
    "message0": "Correct bad pixels with low sigma %1 and high sigma %2",
    "args0": [
      {
        "type": "field_slider",
        "name": "sigmaLow",
        "value" : 3,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      },
      {
        "type": "field_slider",
        "name": "sigmaHigh",
        "value" : 5,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      }
    ],
    "message1": "optionally aware of bayer pattern %1",
    "args1": [
      {
        "type": "input_statement", 
        "name": "debayer"
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_debayer",
    "tooltip": "Extract a single color channel from a bayer mask image, interpolating values",
    "message0": "Extract %1 color channel from bayer mask %2",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "channel",
        "options" : [
          [ "no", ""],
          [ "red", "R"],
          [ "green", "G"],
          [ "blue", "B"]
        ]
      },
      {
        "type": "field_dropdown",
        "name": "colorFilterArray",
        "options" : [
          [ "RGGB", "RGGB"],
          [ "GRBG", "GRBG"],
          [ "GBRG", "GBRG"],
          [ "BGGR", "BGGR"]
        ]
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_debandVert",
    "tooltip": "Apply vertical debanding to reduce chip readout artifacts",
    "message0": "Deband vertically with %1th percentile and window size %2",
    "args0": [
      {
        "type": "field_slider",
        "name": "percentile",
        "value" : 50,
        "min" : 0,
        "max" : 100,
        "precision" : 0.5,
      },
      {
        "type": "field_dropdown",
        "name": "window",
        "options" : [
          [ "8", "8"],
          [ "16", "16"],
          [ "32", "32"],
          [ "64", "64"],
          [ "96", "96"],
          [ "128", "128"],
          [ "192", "192"],
          [ "256", "256"],
          [ "384", "384"],
          [ "512", "512"]
        ]
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_debandHoriz",
    "tooltip": "Apply horizontal debanding to reduce chip readout artifacts",
    "message0": "Deband horizontally with %1th percentile and window size %2",
    "args0": [
      {
        "type": "field_slider",
        "name": "percentile",
        "value" : 50,
        "min" : 0,
        "max" : 100,
        "precision" : 0.5,
      },
      {
        "type": "field_dropdown",
        "name": "window",
        "options" : [
          [ "8", "8"],
          [ "16", "16"],
          [ "32", "32"],
          [ "64", "64"],
          [ "96", "96"],
          [ "128", "128"],
          [ "192", "192"],
          [ "256", "256"],
          [ "384", "384"],
          [ "512", "512"]
        ]
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_scaleOffset",
    "tooltip": "Multiply pixel values with given scale and add given offset",
    "message0": "Multiply by %1 and add %2",
    "args0": [
      {
        "type": "field_slider",
        "name": "scale",
        "value" : 1,
        "min" : 0,
        "max" : 10,
        "precision" : 0.05,
      },
      {
        "type": "field_slider",
        "name": "offset",
        "value" : 0,
        "min" : -10000,
        "max" : 10000,
        "precision" : 50,
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_bin",
    "tooltip": "Add every NxN pixels to reduce noise and image size",
    "message0": "Bin every %1 pixels",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "binSize",
        "options" : [
          [ "1", "1"],
          [ "2", "2"],
          [ "3", "3"],
          [ "4", "4"]
        ]
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_backExtract",
    "tooltip": "Extract background gradient from an image",
    "message0": "Extract background with %1 pixel grid",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "grid",
        "options" : [
          [ "0", "0"],
          [ "32", "32"],
          [ "64", "64"],
          [ "128", "128"],
          [ "256", "256"],
          [ "512", "512"],
          [ "1024", "1024"]
        ]
      }
    ],
    "message1": "masking out stars with %1x their HFR",
    "args1": [
      {
        "type": "field_slider",
        "name": "hfrFactor",
        "value" : 4,
        "min" : 0,
        "max" : 10,
        "precision" : 0.1,
      }
    ],
    "message2": "ignoring pixels %1 sigma above background",
    "args2": [
      {
        "type": "field_slider",
        "name": "sigma",
        "value" : 1,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      }
    ],
    "message3": "and clipping away the brightest %1 cells",
    "args3": [
      {
        "type": "field_slider",
        "name": "clip",
        "value" : 0,
        "min" : 0,
        "max" : 64,
        "precision" : 1,
      }
    ],
    "message4": "optionally saving the background to %1",
    "args4": [
      {
        "type": "input_statement", 
        "name": "save"
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },

  {
    "type": "nl_pre_starDetect",
    "tooltip": "Detect stars based on bright pixels, HFR calculation and\
                brightness ratios inside and outside the HFR.",
    "message0": "Detect stars inside a %1 pixel radius",
    "args0": [
      {
        "type": "field_slider",
        "name": "radius",
        "value" : 16,
        "min" : 0,
        "max" : 128,
        "precision" : 1,
      }
    ],
    "message1": "starting with bright pixels %1 sigma above background",
    "args1": [
      {
        "type": "field_slider",
        "name": "sigma",
        "value" : 8,
        "min" : 0,
        "max" : 20,
        "precision" : 0.1,
      }
    ],
    "message2": "optionally discarding bad pixels %1 sigma above local mean",
    "args2": [
      {
        "type": "field_slider",
        "name": "badPixelSigma",
        "value" : 0,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      }
    ],
    "message3": "keeping stars whose inside is %1x brighter than their outside",
    "args3": [
      {
        "type": "field_slider",
        "name": "inOutRatio",
        "value" : 8,
        "min" : 0,
        "max" : 20,
        "precision" : 0.1,
      }
    ],
    "message4": "optionally saving star detections to %1",
    "args4": [
      {
        "type": "input_statement", 
        "name": "save"
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "pre_blocks",
  },


  // Reference operators
  //
  {
    "type": "nl_ref_selectReference",
    "tooltip": "Select reference frame for histogram normalization and alignment",
    "message0": "Select reference frame by %1",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "mode",
        "options" : [
          [ "highest # stars / HFR (for lights)", "0"],
          [ "median skyfog location (for flats)", "1"],
          [ "given filename", "2"],
          [ "given in-memory image", "3"],
          [ "Lum if present, else best RGB", "4"]
        ]
      }
    ],
    "message1": "with optional filename %1",
    "args1": [
      {
        "type": "field_input",
        "name": "fileName",
        "text": "ref.fits",
      }
    ],
    "message2": "detecting stars with %1",
    "args2": [
      {
        "type": "input_statement", 
        "name": "starDetect"
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "ref_blocks",
  },

  // Postprocessing operators
  //
  {
    "type": "nl_post_matchHistogram",
    "tooltip": "Shift and/or stretch pixel values to match the reference histogram",
    "message0": "Match reference histogram %1",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "mode",
        "options" : [
          [ "disabled", "0"],
          [ "location (for calibration frames)", "1"],
          [ "location and scale (for light frames)", "2"],
          [ "black point (for RGB combination)", "3"]
           // FIXME: auto?
        ]
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "post_blocks",
  },

  {
    "type": "nl_post_align",
    "tooltip": "Align image to reference frame based on star matching",
    "message0": "Align to reference frame with %1 star triangles",
    "args0": [
      {
        "type": "field_slider",
        "name": "k",
        "value" : 50,
        "min" : 0,
        "max" : 200,
        "precision" : 1,
      }
    ],
    "message1": "discarding frames with residuals above %1",
    "args1": [
      {
        "type": "field_slider",
        "name": "threshold",
        "value" : 1,
        "min" : 0,
        "max" : 10,
        "precision" : 0.05,
      }
    ],
    "message2": "replacing out-of-bounds pixels with %1",
    "args2": [
       {
        "type": "field_dropdown",
        "name": "oobMode",
        "options" : [
          [ "not-a-number (for stacking)", "0"],
          [ "the reference skyfog peak", "1"],
          [ "this frame's skyfog peak", "2"]
        ]
      }
    ],

    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "post_blocks",
  },

  // Stacking operators
  //
  {
    "type": "nl_stack_stack",
    "tooltip": "Stack previously aligned images to improve signal-to-noise ratio",
    "message0": "Stack a batch of frames using %1",
    "args0": [
       {
        "type": "field_dropdown",
        "name": "mode",
        "options" : [
          [ "median (no sigmas)", "0"],
          [ "mean (no sigmas)", "1"],
          [ "sigma-clipped mean", "2"],
          [ "winsorized mean", "3"],
          [ "linear regression fit", "4"],
          [ "automatic mode selection", "5"],
        ]
      }
    ],
    "message1": "discarding pixels %1 sigma below or %2 sigma above the mean",
    "args1": [
      {
        "type": "field_slider",
        "name": "sigmaLow",
        "value" : 2.75,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      },
      {
        "type": "field_slider",
        "name": "sigmaHigh",
        "value" : 2.75,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      }
    ],
    "message2": "weighting each image %1",
    "args2": [
       {
        "type": "field_dropdown",
        "name": "weighting",
        "options" : [
          [ "equally", "0"],
          [ "by exposure time", "1"],
          [ "by inverse noise (lower noise has higher weight)", "2"],
          [ "by inverse HFR (lower HFR has higher weight)", "3"]
        ]
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stack_blocks",
  },

  {
    "type": "nl_stack_stackBatches",
    "tooltip": "Create batches fitting in memory, stack each batch with the given\
                operator, then stack the batches with exposure-weighted addition.",
    "message0": "Create batches fitting in memory,",
    "message1": "stack each batch with %1",
    "args1": [
     {
        "type": "input_statement",
        "name": "perBatch"
      }
    ],
    "message2": "and combine the batch stacks weighted by exposure time",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stack_blocks",
  },


  // Stretch operators
  //
  {
    "type": "nl_stretch_normRange",
    "tooltip": "Normalizes pixel values to 0.0 ... 1.0 to enable stretching,\
                gamma and black point correction, color processing and more",
    "message0": "Normalize pixel values",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },
 
  {
    "type": "nl_stretch_stretch",
    "tooltip": "Iteratively applies gamma and black point correction until the peak \
                and the width of the skyfog match target",
    "message0": "Stretch image until skyfog location is %1 and scale is %2",
    "args0": [
      {
        "type": "field_slider",
        "name": "location",
        "value" : 0.1,
        "min" : 0,
        "max" : 1,
        "precision" : 0.005,
      },
      {
        "type": "field_slider",
        "name": "scale",
        "value" : 0.004,
        "min" : 0,
        "max" : 0.1,
        "precision" : 0.001,
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },

  {
    "type": "nl_stretch_midtones",
    "tooltip": "Applies midtone correction, with grey and black level as a multiple \
                of the skyfog scale",
    "message0": "Correct midtones to %1 and black to %2 skyfog scales",
    "args0": [
      {
        "type": "field_slider",
        "name": "mid",
        "value" : 0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
      {
        "type": "field_slider",
        "name": "black",
        "value" : 1,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },

  {
    "type": "nl_stretch_gamma",
    "tooltip": "Applies gamma correction. Values greater than one make the image brighter.\
                Values smaller than one make it darker.",
    "message0": "Adjust image brightness with gamma %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "gamma",
        "value" : 2.0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },

  {
    "type": "nl_stretch_gammaPP",
    "tooltip": "Applies gamma correction to signal pixels, leaving alone skyfog noise pixels",
    "message0": "Correct image brightness with gamma %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "gamma",
        "value" : 2.0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
     ],
    "message1": "for pixels %1 skyfog scales right of the peak",
    "args1": [
     {
        "type": "field_slider",
        "name": "sigma",
        "value" : 1.0,
        "min" : -5,
        "max" : 5,
        "precision" : 0.05,
      },
     ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },

  {
    "type": "nl_stretch_scaleBlack",
    "tooltip": "Shifts the black point to move the skyfog peak to the desired absolute value",
    "message0": "Shift black point to move the skyfog location to %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "location",
        "value" : 0.1,
        "min" : 0,
        "max" : 1,
        "precision" : 0.005,
      },
     ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },

  {
    "type": "nl_stretch_unsharpMask",
    "tooltip": "Increases image sharpness by subtracting a blurred version of the image",
    "message0": "Apply unsharp mask with %1 pixel Gaussian and gain %2",
    "args0": [
      {
        "type": "field_slider",
        "name": "sigma",
        "value" : 1.5,
        "min" : 0,
        "max" : 10,
        "precision" : 0.05,
      },
      {
        "type": "field_slider",
        "name": "gain",
        "value" : 1.0,
        "min" : 0,
        "max" : 1,
        "precision" : 0.01,
      },
     ],
    "message1": "for pixels %1 skyfog scales right of the peak",
    "args1": [
      {
        "type": "field_slider",
        "name": "threshold",
        "value" : 1.0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
     ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },


  // RGB operators
  //
  {
    "type": "nl_rgb_rgbCombine",
    "tooltip": "Combines three mono images into an RGB color image. If a fourth\
                image is present, it is stored in the processing context as a\
                luminance channel for future combination. Output is the RGB image.",
    "message0": "Combine RGB channels",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "rgb_blocks",
  },
 
  {
    "type": "nl_rgb_rgbBalance",
    "tooltip": "Automatically balances colors so skyfog peak locations line up,\
                and average star colors become neutral",
    "message0": "Auto-balance RGB channels",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "rgb_blocks",
  },
 
  {
    "type": "nl_rgb_rgbToHSLuv",
    "tooltip": "Performs a color space conversion. The HSLuv color space is\
                perceptually uniform and allows to modify hue, saturation and\
                luminance independently from each other.",
    "message0": "Convert RGB to HSLuv",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "rgb_blocks",
  },

  {
    "type": "nl_rgb_hsluvToRGB",
    "tooltip": "Performs a color space conversion",
    "message0": "Convert HSLuv to RGB",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "rgb_blocks",
  },

 
  // HSL operators
  //
  {
    "type": "nl_hsl_hslApplyLum",
    "tooltip": "Applies the luminance channel stored in the processing context \
                to the current HSLuv image",
    "message0": "Apply luminance channel",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },
 
  {
    "type": "nl_hsl_hslScaleOffsetChannel",
    "tooltip": "Multiply pixel values in given channel with given scale and add given offset",
    "message0": "Multiply channel %1 by %2 and add %3",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "channelID",
        "options" : [
          [ "Hue", "0"],
          [ "Saturation", "1"],
          [ "Luminance", "2"]
        ]
      },
      {
        "type": "field_slider",
        "name": "scale",
        "value" : 1,
        "min" : 0,
        "max" : 10,
        "precision" : 0.05,
      },
      {
        "type": "field_slider",
        "name": "offset",
        "value" : 0,
        "min" : -0.5,
        "max" : 0.5,
        "precision" : 0.005,
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },



  {
    "type": "nl_hsl_hslNeutralizeBackground",
    "tooltip": "For luminances sigLow standard deviations above the background peak, bring saturation to zero.\
                For luminances above sigHigh, keep full saturation. For those in between, interpolate linearly.",
    "message0": "Desaturate pixels darker than %1 sigmas",
    "args0": [
      {
        "type": "field_slider",
        "name": "sigmaLow",
        "value" : 0.5,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
    ],
   "message1": "keep saturation above %1 sigmas",
    "args1": [
      {
        "type": "field_slider",
        "name": "sigmaHigh",
        "value" : 0.75,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      }      
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },
 
  {
    "type": "nl_hsl_hslSaturationGamma",
    "tooltip": "Boost saturation with a gamma curve. Applied selectively to pixels whose luminance\
                is a given number of standard deviations brighter than the skyfog peak location.",
    "message0": "Apply gamma %1 to saturation",
    "args0": [
      {
        "type": "field_slider",
        "name": "gamma",
        "value" : 1.5,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
    ],
    "message1": "for luminance above %1 sigma",
    "args1": [
      {
        "type": "field_slider",
        "name": "sigma",
        "value" : 1.0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      }      
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },
 
  {
    "type": "nl_hsl_hslSelectiveSaturation",
    "tooltip": "Multiplies saturation by the given factor, for hues in the given range.\
                Can be used to e.g. remove purple star colors",
    "message0": "Multiply saturation by %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "factor",
        "value" : 0.5,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
    ],
    "message1": "for hues between %1 and %2",
    "args1": [
      {
        "type": "field_angle",
        "name": "from",
        "angle" : 295,
      },      
      {
        "type": "field_angle",
        "name": "to",
        "angle" : 40,
      }      
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

  {
    "type": "nl_hsl_hslRotateHue",
    "tooltip": "Shift color hues in the given range by the given amount.\
                Applied selectively where luminance is brighter than the skyfog peak location\
                by the given amount of standard deviations.\
                This is useful for creating Hubble palette images by turning greens to yellows.",
    "message0": "Rotate hues between %1 and %2",
    "args0": [
      {
        "type": "field_angle",
        "name": "from",
        "angle" : 100,
      },      
      {
        "type": "field_angle",
        "name": "to",
        "angle" : 190,
      },      
    ],
    "message1": "by %1 for luminances above %2 sigma",
    "args1": [
      {
        "type": "field_slider",
        "name": "offset",
        "value" : 35,
        "min" : -180,
        "max" : 180,
        "precision" : 1,
      },
      {
        "type": "field_slider",
        "name": "sigma",
        "value" : 0.75,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },    
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

  {
    "type": "nl_hsl_hslSCNR",
    "tooltip": "Selectively reduces chroma noise in the green channel by the given amount.\
                This is useful for creating Hubble palette images, after applying a color rotation.",
    "message0": "SCNR %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "factor",
        "value" : 0.5,
        "min" : 0,
        "max" : 1,
        "precision" : 0.01,
      },
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

  {
    "type": "nl_hsl_hslMidtones",
    "tooltip": "Applies midtone correction, with grey and black level as a multiple of the skyfog scale",
    "message0": "Correct luminance midtones to %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "mid",
        "value" : 0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
    ],
    "message1": "and black to %1 skyfog scales",
    "args1": [
      {
        "type": "field_slider",
        "name": "black",
        "value" : 1,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

  {
    "type": "nl_hsl_hslGamma",
    "tooltip": "Applies gamma correction. Values greater than one make the image brighter.\
                Values smaller than one make it darker.",
    "message0": "Adjust luminance brightness with gamma %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "gamma",
        "value" : 2.0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

  {
    "type": "nl_hsl_hslGammaPP",
    "tooltip": "Applies gamma correction to signal pixels, leaving alone skyfog noise pixels",
    "message0": "Adjust luminance brightness with gamma %1",
    "args0": [
      {
        "type": "field_slider",
        "name": "gamma",
        "value" : 2.0,
        "min" : 0,
        "max" : 5,
        "precision" : 0.01,
      },
     ],
    "message1": "for pixels %1 skyfog scales right of the peak",
    "args1": [
     {
        "type": "field_slider",
        "name": "sigma",
        "value" : 1.0,
        "min" : -5,
        "max" : 5,
        "precision" : 0.05,
      },
     ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

  {
    "type": "nl_hsl_hslScaleBlack",
    "tooltip": "Shifts the black point to move the skyfog peak to the desired absolute value",
    "message0": "Shift luminance channel black point",
    "message1": "to move the skyfog location to %1",
    "args1": [
      {
        "type": "field_slider",
        "name": "location",
        "value" : 0.1,
        "min" : 0,
        "max" : 1,
        "precision" : 0.005,
      },
     ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "hsl_blocks",
  },

 ]);

Blockly.FieldAngle.CLOCKWISE = true // Blockly angle picker direction, unfortunately global
Blockly.FieldAngle.OFFSET    =  90  // Blockly angle picker zero direction in degrees, unfortunately global
Blockly.FieldAngle.ROUND     =   1  // Blockly angle resolution in degrees, unfortunately global
Blockly.FieldAngle.HALF      =  64  // Blockly angle picker sizes in pixels, unfortunately global

