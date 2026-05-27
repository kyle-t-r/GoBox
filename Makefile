.PHONY: build clean

build:
	mkdir -p output/static output/publishers/disk-monitor output/publishers/sys-monitor
	cd gobox && go build -o ../output/gobox
	cd publishers/disk-monitor && go build -o ../../output/publishers/disk-monitor/disk-monitor
	cd publishers/sys-monitor && go build -o ../../output/publishers/sys-monitor/sys-monitor
	cp -r gobox/static/* output/static/; if [ ! -f output/config.yaml ]; then touch output/config.yaml; fi

clean:
	rm -rf output/publishers
	rm -rf output/static
	rm output/gobox
