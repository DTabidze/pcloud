// +build !ignore_autogenerated

// gen

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaCA) DeepCopyInto(out *NebulaCA) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaCA.
func (in *NebulaCA) DeepCopy() *NebulaCA {
	if in == nil {
		return nil
	}
	out := new(NebulaCA)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaCA) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaCAList) DeepCopyInto(out *NebulaCAList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]NebulaCA, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaCAList.
func (in *NebulaCAList) DeepCopy() *NebulaCAList {
	if in == nil {
		return nil
	}
	out := new(NebulaCAList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *NebulaCAList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaCASpec) DeepCopyInto(out *NebulaCASpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaCASpec.
func (in *NebulaCASpec) DeepCopy() *NebulaCASpec {
	if in == nil {
		return nil
	}
	out := new(NebulaCASpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NebulaCAStatus) DeepCopyInto(out *NebulaCAStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NebulaCAStatus.
func (in *NebulaCAStatus) DeepCopy() *NebulaCAStatus {
	if in == nil {
		return nil
	}
	out := new(NebulaCAStatus)
	in.DeepCopyInto(out)
	return out
}
