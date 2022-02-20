JSONS  =$(wildcard *.json)
RESULTS=$(patsubst %.json,%.out,$(JSONS))
DIFFS  =$(patsubst %.json,%.diff,$(JSONS))

all: start tests stop

clean:
	rm -f $(RESULTS) $(DIFFS) serve.log
start:
	killall -q nightlight ; (../nightlight serve > serve.log 2>&1 &) ; sleep 2

stop:
	killall -q nightlight

tests: $(DIFFS)
	cat $(DIFFS)

%.out:	%.json ../nightlight*
	curl -H 'Content-Type: application/json' -d '@$<' http://127.0.0.1:8080/api/v1/$* > $@

%.diff: %.out
	diff $< $*.expected > $@

