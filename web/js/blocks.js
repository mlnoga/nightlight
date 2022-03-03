Blockly.defineBlocksWithJsonArray([
  // File operators
  //

  {
    "type": "nl_file_load",
    "message0": "Load single image %1",
    "tooltip": "Load a FITS image from a file",
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
    "message0": "Load many images %1",
    "tooltip": "Load many FITS images from a filename pattern with wildcards * and ?",
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
    "message0": "Save image to %1",
    "tooltip": "Save image to FITS or JPEG based on extension, expanding %d to the image ID",
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
    "message0": "Calibrate with dark %1 and with flat %2",
    "tooltip": "Calibrate image with a (master) dark frame and a (master) flat frame",
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
    "message0": "Correct bad pixels with low sigma %1 and high sigma %2",
    "tooltip": "Cosmetic correction of pixels whose value is more than a given number of standard deviations, or sigmas, away from the local mean",
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
    "message0": "Extract %1 color channel from bayer mask %2",
    "tooltip": "Extract a single color channel from a bayer mask image, interpolating values",
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
    "type": "nl_pre_bin",
    "message0": "Bin every %1 pixels",
    "tooltip": "Add every NxN pixels to reduce noise and image size",
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
    "message0": "Extract background with %1 pixel grid",
    "tooltip": "Extract background gradient from an image",
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
    "message1": "ignoring pixels %1 sigma above background",
    "args1": [
      {
        "type": "field_slider",
        "name": "sigma",
        "value" : 1,
        "min" : 0,
        "max" : 6,
        "precision" : 0.01,
      }
    ],
    "message2": "and clipping away the brightest %1 cells",
    "args2": [
      {
        "type": "field_slider",
        "name": "clip",
        "value" : 0,
        "min" : 0,
        "max" : 64,
        "precision" : 1,
      }
    ],
    "message3": "optionally saving the background to %1",
    "args3": [
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
    "message0": "Detect stars inside a %1 pixel radius",
    "tooltip": "Detect stars based on bright pixels and HFR",
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
    "message0": "Select reference frame by %1",
    "tooltip": "Select reference frame for histogram normalization and alignment",
    "args0": [
      {
        "type": "field_dropdown",
        "name": "mode",
        "options" : [
          [ "highest # stars / HFR (for lights)", "0"],
          [ "median skyfog location (for flats)", "1"],
          [ "given filename", "2"],
          [ "given in-memory image", "3"] // FIXME: still needed?
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
    "message0": "Match reference histogram %1",
    "tooltip": "Shift and/or stretch pixel values to match the reference histogram",
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
    "message0": "Align to reference frame with %1 star triangles",
    "tooltip": "Align image to reference frame based on star matching",
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
    "message0": "Stack a batch of frames using %1",
    "tooltip": "Stack previously aligned images to improve signal-to-noise ratio",
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
    "message0": "Create batches fitting in memory,",
    "tooltip": "Stack previously aligned images to improve signal-to-noise ratio",
    "message1": "stack each batch with %1",
    "args1": [
     {
        "type": "input_statement",
        "name": "perBatch"
      }
    ],
    "message2": "create stack-of-batches weighted by exposure time",
    "message3": "detect stars using %1",
    "args3": [
     {
        "type": "input_statement",
        "name": "starDetect"
      }
    ],
    "message4": "and save final result to %1",
    "args4": [
     {
        "type": "input_statement",
        "name": "save"
      }
    ],
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stack_blocks",
  },


  // Stretch operators
  //
  {
    "type": "nl_stretch_normRange",
    "message0": "Normalize pixel values",
    "tooltip": "Normalizes pixel values to 0.0 ... 1.0 to enable gamma correction, color processing and more",
    "previousStatement" : null,
    "nextStatement" : null,
    "style"  : "stretch_blocks",
  },
 
  {
    "type": "nl_stretch_stretch",
    "message0": "Stretch image until skyfog location is %1 and scale is %2",
    "tooltip": "Iteratively applies gamma and black point correction until the peak and the width of the skyfog match target",
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
    "message0": "Correct midtones to %1 and black to %2 skyfog scales",
    "tooltip": "Applies midtone correction, with grey and black level as a multiple of the skyfog scale",
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
    "message0": "Correct image brightness with gamma %1",
    "tooltip": "Applies gamma correction, values greater one increase brightness",
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
    "message0": "Correct image brightness with gamma %1",
    "tooltip": "Applies gamma correction to signal pixels, leaving alone skyfog noise pixels",
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
    "message0": "Shift black point to move the skyfog location to %1",
    "tooltip": "Shifts the black point to move the skyfog peak to the desired absolute value",
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
    "message0": "Apply unsharp mask with %1 pixel Gaussian and gain %2",
    "tooltip": "Increases image sharpness by subtracting a blurred version of the image",
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


]);

