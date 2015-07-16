FROM scratch
MAINTAINER Albert Dixon <albert.dixon@schange.com>

ADD /bin/romulusd-linux /romulusd
ENTRYPOINT ["/romulusd"]
