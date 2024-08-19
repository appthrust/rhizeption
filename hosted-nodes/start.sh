docker run \
    --privileged \
    --pid=host \
    --network=host \
    --cgroupns=host \
    --rm -it hn:latest
