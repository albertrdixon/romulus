FROM scratch
MAINTAINER Albert Dixon <albert.dixon@schange.com>
ENTRYPOINT ["/romulusd"]
ADD /bin/romulusd-linux /romulusd
