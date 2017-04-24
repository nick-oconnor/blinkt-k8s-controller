# A Kubernetes Controller for the Pimoroni Blinkt on a Raspberry Pi #

<img src="https://github.com/apprenda/blinkt-k8s-controller/raw/master/images/rpi-minicluster.jpg" width="300" />

A simple way to physically/visually display the number of Pods running on Raspberry Pi-based Kubernetes worker nodes by using a [Pimoroni Blinkt](https://shop.pimoroni.com/products/blinkt).

The Blinkt is a low-profile strip of eight super-bright, color LED indicators that plugs directly onto the Raspberry Pi's GPIO header. Several available software libraries make it easy to control the color and brightness of each LED independently.

## How It Works ##

This controller is designed to be deployed as a [DaemonSet](https://kubernetes.io/docs/admin/daemons/) to control Blinkt devices connected to Raspberry Pi Kubernetes worker nodes. Once deployed, every Pod with a label of `blinkt: show` that lands on a node will cause an LED indicator on that node's Blinkt to turn on (only the first 8 Pods can be displayed). As new Pods get created or deleted the light display will adjust accordingly. The color of the indicator can be customized by editing the `COLOR` environment variable in the included sample deployment file. 

## Acknowledgements ##

This project draws inspiration and borrows heavily from the work done by @alexellis on [Docker on Raspberry Pis](http://blog.alexellis.io/visiting-pimoroni/) and his [Blinkt Go libraries](https://github.com/alexellis/blinkt_go), themselves based on work by @gamaral for using the `/sys/` fs interface [instead of special libraries or elevated privileges](https://guillermoamaral.com/read/rpi-gpio-c-sysfs/) to `/dev/mem` on the Raspberry Pi.

## Requirements ##

A Raspberry Pi-based Kubernetes cluster. Hypriot has a good [blog post](https://blog.hypriot.com/post/setup-kubernetes-raspberry-pi-cluster/) that describes a good way to set this up. 

Physically install a [Pimoroni Blinkt](https://shop.pimoroni.com/products/blinkt) on all the Raspberry Pi worker nodes you want to use for display. **No additional sofware or setup is required for the Blinkt**.

*Note: For greater control, every node equiped with a Blinkt should be labeled with `deviceType: blinkt`. A [nodeSelector](https://kubernetes.io/docs/admin/daemons/#running-pods-on-only-some-nodes) can then be used to insure that only labeled nodes run the Controller Pod. For example:*

```sh
kubectl label node <nodename> deviceType=blinkt
```

*If you want to run the Pod on all nodes you can skip this step and remove the `nodeSelector` from the DaemonSet Descriptor*

## Usage ##

If you're running on a Kubernetes v1.6 cluster or above and have RBAC enabled, you can use the sample RBAC Descriptor to run the DaemonSet with the proper permissions:

```sh
kubectl create -f kubernetes/blinkt-k8s-controller-rbac.yaml
```

Create the DaemonSet using the included Resource Descriptor:

```sh
kubectl create -f kubernetes/blinkt-k8s-controller-ds.yaml
```

Label Pods with `blinkt: show` to have them show up in the Blinkt. For example:
```yaml
kind: Deployment
apiVersion: extensions/v1beta1
metadata:
  ...
spec:
  template:
    metadata:
      labels:
        ...
        blinkt: show
    spec:
      ...
```

## Building Your Own ##

You need a properly configured [Go environment](https://golang.org) and the [Glide](https://glide.sh) vendoring command. Just edit the `main.go` file and run:

```sh
glide update --strip-vendor
./build.sh
```

to cross-compile to a local binary named `main`. You can then edit and run the `dockerize.sh` script to create your own Docker Image repository and tags. You then have to upload it to your container registry of choice and modify the DaemonSet Descriptor to use your new Image Repo instead.

## License ##

This software is made available under an Apache License, Version 2.0. See [LICENSE](./LICENSE).