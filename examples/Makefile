# General settings
OBJ =YOUR_OBJECT_NAME

# Example for Hubble Palette
CHAN=HaS2O3# RGB, aRGB, LRGB, HaRGB, S2HaO3, HaS2O3 or HaO3O3. No spaces before the #, please
COMBINE=-chroma 1.1 -rotFrom 120 -rotTo 147.5 -rotBy -35 -scnr 0.5 -autoScale 0.5 -ppGamma 1.7

# Example for LRGB
#CHAN=LRGB
#RGB_BINNING=-binning 2
#COMBINE=-chroma 1.5 -autoScale 0.2 -ppGamma 2.7

# Provide paths to your master dark frames here (they depend on exposure length, temperature, gain and bias)
DARKL =calib/dark_L.fits
DARKR =calib/dark_RGB.fits
DARKG =calib/dark_RGB.fits
DARKB =calib/dark_RGB.fits
DARKHa=calib/dark_Ha.fits
DARKO3=calib/dark_O3.fits
DARKS2=calib/dark_S2.fits

# Provide paths to your master flat frames here (they mostly depend on the filter, as well as on gain and bias)
FLATL =calib/flat_L.fits
FLATR =calib/flat_R.fits
FLATG =calib/flat_G.fits
FLATB =calib/flat_B.fits
FLATHa=calib/flat_Ha.fits
FLATO3=calib/flat_O3.fits
FLATS2=calib/flat_S2.fits

# Additional per-channel settings (put e.g. pre-determined stacking sigmas here to avoid the goal seek)PARAML =
PARAMR =
PARAMG =
PARAMB =
PARAMHa=
PARAMO3=
PARAMS2=

# Additional overall settings (typically not necessary)
STD    =

# Set the path to your nightlight executable here
NL     =nightlight


# Makefile targets and rules. These should usually not require any changes

all: $(OBJ)_$(CHAN).fits

backup:
	mkdir -p backup && mv *.fits *.jpg *.log backup/

clean: 
	rm -f $(OBJ)_RGB.fits $(OBJ)_LRGB.fits $(OBJ)_HaS2O3.fits $(OBJ)_HaO3S2.fits $(OBJ)_S2HaO3.fits $(OBJ)_HaO3O3.fits $(OBJ)_HaS2S2.fits \
	$(OBJ)_RGB.jpg $(OBJ)_LRGB.jpg $(OBJ)_HaS2O3.jpg $(OBJ)_HaO3S2.jpg $(OBJ)_S2HaO3.jpg $(OBJ)_HaO3O3.jpg $(OBJ)_HaS2S2.jpg \
	$(OBJ)_RGB.log $(OBJ)_LRGB.log $(OBJ)_HaS2O3.log $(OBJ)_HaO3S2.log $(OBJ)_S2HaO3.log $(OBJ)_HaO3O3.log $(OBJ)_HaS2S2.log 

realclean: clean
	rm -f $(OBJ)_L.fits $(OBJ)_R.fits $(OBJ)_G.fits $(OBJ)_B.fits \
	$(OBJ)_Ha.fits $(OBJ)_O3.fits $(OBJ)_S2.fits \
	$(OBJ)_L.log $(OBJ)_R.log $(OBJ)_G.log $(OBJ)_B.log \
	$(OBJ)_Ha.log $(OBJ)_O3.log $(OBJ)_S2.log 

folders:
	for f in *_L_*.fits; do if test -f "$$f";  then mkdir -p L;  mv *_L_*.fits  L/;  fi; break; done
	for f in *_R_*.fits; do if test -f "$$f";  then mkdir -p R;  mv *_R_*.fits  R/;  fi; break; done
	for f in *_G_*.fits; do if test -f "$$f";  then mkdir -p G;  mv *_G_*.fits  G/;  fi; break; done
	for f in *_B_*.fits; do if test -f "$$f";  then mkdir -p B;  mv *_B_*.fits  B/;  fi; break; done
	for f in *_Ha_*.fits; do if test -f "$$f"; then mkdir -p Ha; mv *_Ha_*.fits Ha/; fi; break; done
	for f in *_O3_*.fits; do if test -f "$$f"; then mkdir -p O3; mv *_O3_*.fits O3/; fi; break; done
	for f in *_S2_*.fits; do if test -f "$$f"; then mkdir -p S2; mv *_S2_*.fits S2/; fi; break; done

count:
	for f in L R G B Ha O3 S2; do if test -e "$$f"; then echo "$$f" has `ls $$f | wc -l` frames; fi ; done

%.stats: %.fits 
	$(NL) $(STD) $(STATS) -log $@ stats $^

%_S2HaO3.fits: %_S2.fits %_Ha.fits %_O3.fits
	$(NL) $(STD) $(COMBINE) -out $@ rgb $^

%_HaS2O3.fits: %_Ha.fits %_S2.fits %_O3.fits
	$(NL) $(STD) $(COMBINE) -out $@ rgb $^

%_HaO3S2.fits: %_Ha.fits %_O3.fits %_S2.fits 
	$(NL) $(STD) $(COMBINE) -out $@ rgb $^

%_HaO3O3.fits: %_Ha.fits %_O3.fits
	$(NL) $(STD) $(COMBINE) -out $@ rgb $^ $*_O3.fits

%_HaS2S2.fits: %_Ha.fits %_S2.fits
	$(NL) $(STD) $(COMBINE) -out $@ rgb $^ $*_S2.fits

%_RGB.fits: %_R.fits %_G.fits %_B.fits
	$(NL) $(STD) $(COMBINE) -out $@ rgb $^

%_aRGB.fits: %_L.fits %_R.fits %_G.fits %_B.fits
	$(NL) $(STD) $(COMBINE) -out $@ argb $^

%_LRGB.fits: %_L.fits %_R.fits %_G.fits %_B.fits
	$(NL) $(STD) $(COMBINE) -out $@ lrgb $^

%_HaRGB.fits: %_Ha.fits %_R.fits %_G.fits %_B.fits
	$(NL) $(STD) $(COMBINE) -out $@ lrgb $^

$(OBJ)_L.fits: L/*.fits 
	$(NL) $(STD) $(PARAML) -dark $(DARKL) -flat $(FLATL) -out $@ stack "L/*.fits"

$(OBJ)_R.fits: R/*.fits 
	$(NL) $(STD) $(PARAMR) $(RGB_BINNING) -dark $(DARKR) -flat $(FLATR) -out $@ stack "R/*.fits"

$(OBJ)_G.fits: G/*.fits 
	$(NL) $(STD) $(PARAMG) $(RGB_BINNING) -dark $(DARKG) -flat $(FLATG) -out $@ stack "G/*.fits"

$(OBJ)_B.fits: B/*.fits 
	$(NL) $(STD) $(PARAMB) $(RGB_BINNING) -dark $(DARKB) -flat $(FLATB) -out $@ stack "B/*.fits"

$(OBJ)_Ha.fits: Ha/*.fits 
	$(NL) $(STD) $(PARAMHa) -dark $(DARKHa) -flat $(FLATHa) -out $@ stack "Ha/*.fits"

$(OBJ)_O3.fits: O3/*.fits 
	$(NL) $(STD) $(PARAMO3) -dark $(DARKO3) -flat $(FLATO3) -out $@ stack "O3/*.fits"

$(OBJ)_S2.fits: S2/*.fits 
	$(NL) $(STD) $(PARAMS2) -dark $(DARKS2) -flat $(FLATS2) -out $@ stack "S2/*.fits"
