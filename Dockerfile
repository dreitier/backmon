FROM fedora:30
WORKDIR /usr/local/bin

# copy binaries from our local work directory
COPY cloudmon .
# interpolator has been build by Dockerfile.build
# @see https://github.com/dreitier/interpolator
COPY interpolator .
COPY entrypoint.sh .

EXPOSE 8000/tcp

ENTRYPOINT ["entrypoint.sh"]