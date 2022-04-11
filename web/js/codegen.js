// Code generation for nightlight JSON specs
//
const Json = new Blockly.Generator("Json")

// Turns sequential statements into seq objects
Json.scrub_ = function(block, code, opt_thisOnly) {
  if(opt_thisOnly)
    return code;
  var nextBlock = block.nextConnection && block.nextConnection.targetBlock();
  if(!nextBlock)
    return code;
  var steps=[JSON.parse(code)];
  while(nextBlock) {
    const nextString=this.blockToCode(nextBlock, true);
    if(nextString!="") // block might be disabled
      steps.push(JSON.parse(nextString));
    block=nextBlock;
    nextBlock = block.nextConnection && block.nextConnection.targetBlock();
  }
  const seq={"type":"seq", steps: steps}
  return JSON.stringify(seq);
};

// Turns a block representing a Nightlight operator into stringified JSON 
function createJsonObject(block, typeName, fieldNames, numberFieldNames, statementNames) {
  var res={"type": typeName};
  if(fieldNames)
    fieldNames.forEach((fieldName, index) => {
      res[fieldName]=block.getFieldValue(fieldName);
    });
  if(numberFieldNames)
    numberFieldNames.forEach((fieldName, index) => {
      res[fieldName]=parseInt(block.getFieldValue(fieldName));
    });
  if(statementNames)
    statementNames.forEach((statementName, index) => {
      const statementString=Json.statementToCode(block,statementName);
      const statement=statementString=="" ? null : JSON.parse(statementString);
      res[statementName]=statement;
    });
  return JSON.stringify(res);
}

Json["nl_file_load"]=function(block) {
  return createJsonObject(block, "load", ["fileName"], null, null);
}

Json["nl_file_loadMany"]=function(block) {
  var res={"type": "loadMany", 
           "filePatterns" : [ block.getFieldValue("filePattern") ],
          };
  return JSON.stringify(res);
  // return createJsonObject(block, "loadMany", ["filePattern"], null);
}

Json["nl_file_save"]=function(block) {
  return createJsonObject(block, "save", ["filePattern"], null, null);
}

Json["nl_pre_calibrate"]=function(block) {
  return createJsonObject(block, "calibrate", ["dark", "flat"], null, null);
}

Json["nl_pre_badPixel"]=function(block) {
  return createJsonObject(block, "badPixel", ["sigmaLow", "sigmaHigh"], null, ["debayer"]);
}

Json["nl_pre_debayer"]=function(block) {
  return createJsonObject(block, "debayer", ["channel", "colorFilterArray"], null, null);
}

Json["nl_pre_debandVert"]=function(block) {
  return createJsonObject(block, "debandVert", ["percentile", "window"], null, null);
}

Json["nl_pre_debandHoriz"]=function(block) {
  return createJsonObject(block, "debandHoriz", ["percentile", "window"], null, null);
}

Json["nl_pre_scaleOffset"]=function(block) {
  return createJsonObject(block, "scaleOffset", ["scale", "offset"], null);
}

Json["nl_pre_bin"]=function(block) {
  return createJsonObject(block, "bin", ["binSize"], null, null);
}

Json["nl_pre_backExtract"]=function(block) {
  return createJsonObject(block, "backExtract", ["gridSize", "sigma", "clip"], null, ["save"]);
}

Json["nl_pre_starDetect"]=function(block) {
  return createJsonObject(block, "starDetect", ["radius", "sigma", "badPixelSigma", "inOutRatio"], null, ["save"]);
}

Json["nl_ref_selectReference"]=function(block) {
  return createJsonObject(block, "selectRef", ["fileName",], ["mode"], ["starDetect"]);
}

Json["nl_post_matchHistogram"]=function(block) {
  return createJsonObject(block, "matchHist", null, ["mode"], null);  
}

Json["nl_post_align"]=function(block) {
  return createJsonObject(block, "align", ["k", "threshold"], ["oobMode"], null);
}

Json["nl_stack_stack"]=function(block) {
  return createJsonObject(block, "stack", ["sigmaLow", "sigmaHigh"], ["mode", "weighting"], null);
}

Json["nl_stack_stackBatches"]=function(block) {
  return createJsonObject(block, "stackBatches", null, null, ["perBatch"]);
}

Json["nl_stretch_normRange"]=function(block) {
  return createJsonObject(block, "normRange", null, null);
}

Json["nl_stretch_stretch"]=function(block) {
  return createJsonObject(block, "stretch", ["location", "scale"], null);
}

Json["nl_stretch_midtones"]=function(block) {
  return createJsonObject(block, "midtones", ["mid", "black"], null);
}

Json["nl_stretch_gamma"]=function(block) {
  return createJsonObject(block, "gamma", ["gamma"], null);
}

Json["nl_stretch_gammaPP"]=function(block) {
  return createJsonObject(block, "gammaPP", ["gamma", "sigma"], null);
}

Json["nl_stretch_scaleBlack"]=function(block) {
  return createJsonObject(block, "scaleBlack", ["location"], null);
}

Json["nl_stretch_unsharpMask"]=function(block) {
  return createJsonObject(block, "unsharpMask", ["sigma", "gain", "threshold"], null);
}

Json["nl_rgb_rgbCombine"]=function(block) {
  return createJsonObject(block, "rgbCombine", null, null);
}

Json["nl_rgb_rgbBalance"]=function(block) {
  return createJsonObject(block, "rgbBalance", null, null);
}

Json["nl_rgb_rgbToHSLuv"]=function(block) {
  return createJsonObject(block, "rgbToHSLuv", null, null);
}

Json["nl_rgb_hsluvToRGB"]=function(block) {
  return createJsonObject(block, "hsluvToRGB", null, null);
}

Json["nl_hsl_hslApplyLum"]=function(block) {
  return createJsonObject(block, "hslApplyLum", null, null);
}

Json["nl_hsl_hslScaleOffsetChannel"]=function(block) {
  return createJsonObject(block, "hslScaleOffsetChannel", ["channelID", "scale", "offset"], null);
}

Json["nl_hsl_hslNeutralizeBackground"]=function(block) {
  return createJsonObject(block, "hslNeutralizeBackground", ["sigmaLow", "sigmaHigh"], null);
}

Json["nl_hsl_hslSaturationGamma"]=function(block) {
  return createJsonObject(block, "hslSaturationGamma", ["gamma", "sigma"], null);
}

Json["nl_hsl_hslSelectiveSaturation"]=function(block) {
  return createJsonObject(block, "hslSelectiveSaturation", ["from", "to", "factor"], null);
}

Json["nl_hsl_hslRotateHue"]=function(block) {
  return createJsonObject(block, "hslRotateHue", ["from", "to", "offset", "sigma"], null);
}

Json["nl_hsl_hslSCNR"]=function(block) {
  return createJsonObject(block, "hslSCNR", ["factor"], null);
}

Json["nl_hsl_hslMidtones"]=function(block) {
  return createJsonObject(block, "hslMidtones", ["mid", "black"], null);
}

Json["nl_hsl_hslGamma"]=function(block) {
  return createJsonObject(block, "hslGamma", ["gamma"], null);
}

Json["nl_hsl_hslGammaPP"]=function(block) {
  return createJsonObject(block, "hslGammaPP", ["gamma", "sigma"], null);
}

Json["nl_hsl_hslScaleBlack"]=function(block) {
  return createJsonObject(block, "hslScaleBlack", ["location"], null);
}

