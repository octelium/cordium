import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as MetaPB from "@octelium/apis/main/metav1";

export interface CommonSpec {
  image?: WsPB.Workspace_Spec_Image;
  runtime?: WsPB.Workspace_Spec_Runtime;
  repository?: WsPB.Workspace_Spec_Repository;
  additionalRepositories: WsPB.Workspace_Spec_AdditionalRepository[];
  limit?: WsPB.Workspace_Spec_Limit;
  vars: WsPB.Workspace_Spec_Var[];
}

export type SpecKind = "Workspace" | "Template";

export interface SectionProps {
  kind: SpecKind;
  spec: CommonSpec;
  spaceRef: MetaPB.ObjectReference;
  patch: (fn: (draft: CommonSpec) => void) => void;
}
