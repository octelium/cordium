import * as WsPB from "@octelium/apis/main/cordiumv1";
import * as React from "react";

import { getClientWorkspace } from "@/utils/client";

import MetadataEdit from "@/components/MetadataEdit";
import { onError } from "@/utils";
import { useAppSelector } from "@/utils/hooks";
import {
  Button,
  PasswordInput,
  SegmentedControl,
  Select,
  Textarea,
} from "@mantine/core";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Eye, EyeOff, FileText, Loader2, Plus, Upload, X } from "lucide-react";
import { toast } from "react-hot-toast";
import { useNavigate } from "react-router-dom";

type InputMode = "value" | "file";

const CreateUserSecret = () => {
  const [req, setReq] = React.useState(
    WsPB.UserSecret.create({
      apiVersion: "cordium/v1",
      kind: "UserSecret",
      metadata: {},
      spec: {},
      status: {},
      data: { type: { oneofKind: "value", value: "" } },
    }),
  );

  const [inputMode, setInputMode] = React.useState<InputMode>("value");
  const [revealed, setRevealed] = React.useState(false);
  const [fileName, setFileName] = React.useState<string | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement | null>(null);

  const client = getClientWorkspace();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useAppSelector((a) => a.settings.status?.user);

  const mutation = useMutation({
    mutationFn: async () => {
      const { response } = await client.createUserSecret(req);
      return response;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["workspace/listUserSecret/"],
      });
      queryClient.invalidateQueries({
        queryKey: ["workspace/listUserSecret", 0],
      });
      toast.success("Secret created successfully");
      navigate("/usersecrets");
    },
    onError,
  });

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setFileName(file.name);
    const reader = new FileReader();
    reader.onload = (ev) => {
      const text = ev.target?.result as string;
      const next = WsPB.UserSecret.clone(req);
      next.data!.type = { oneofKind: "value", value: text };
      setReq(next);
    };
    reader.readAsText(file);
    e.target.value = "";
  };

  const currentValue =
    req.data?.type.oneofKind === "value" ? req.data.type.value : "";

  return (
    <div className="w-full flex flex-col gap-6">
      <input
        type="text"
        autoComplete="username"
        style={{ display: "none" }}
        readOnly
        aria-hidden="true"
      />
      <input
        type="password"
        autoComplete="new-password"
        style={{ display: "none" }}
        readOnly
        aria-hidden="true"
      />

      <div className="bg-white border border-slate-200 rounded-xl shadow-[0_1px_4px_rgba(15,23,42,0.06)]">
        <div className="px-5 py-3.5 border-b border-slate-100 bg-slate-50/60">
          <span className="text-[0.72rem] font-bold uppercase tracking-[0.06em] text-slate-500">
            Metadata
          </span>
        </div>
        <div className="p-5">
          <MetadataEdit
            metadata={req.metadata!}
            onUpdate={(v) => {
              const next = WsPB.UserSecret.clone(req);
              next.metadata = v;
              setReq(next);
            }}
            parentName={user?.metadata?.name}
            skipDisplayName
          />
        </div>
      </div>

      <div className="bg-white border border-slate-200 rounded-xl shadow-[0_1px_4px_rgba(15,23,42,0.06)]">
        <div className="px-5 py-3.5 border-b border-slate-100 bg-slate-50/60">
          <span className="text-[0.72rem] font-bold uppercase tracking-[0.06em] text-slate-500">
            Spec
          </span>
        </div>
        <div className="p-5 flex flex-col gap-5">
          <Select
            label="Type"
            description="Set the type of the secret"
            data={[
              {
                label: "Default",
                value:
                  WsPB.UserSecret_Spec_Type[WsPB.UserSecret_Spec_Type.DEFAULT],
              },
              {
                label: "SSH Key",
                value:
                  WsPB.UserSecret_Spec_Type[WsPB.UserSecret_Spec_Type.SSH_KEY],
              },
            ]}
            value={WsPB.UserSecret_Spec_Type[req.spec!.type]}
            onChange={(val) => {
              const next = WsPB.UserSecret.clone(req);
              next.spec!.type = WsPB.UserSecret_Spec_Type[val as "DEFAULT"];
              setReq(next);
            }}
          />

          {req.spec?.type === WsPB.UserSecret_Spec_Type.DEFAULT && (
            <div className="flex flex-col gap-3">
              <SegmentedControl
                value={inputMode}
                onChange={(v) => {
                  setInputMode(v as InputMode);
                  setFileName(null);
                  const next = WsPB.UserSecret.clone(req);
                  next.data!.type = { oneofKind: "value", value: "" };
                  setReq(next);
                }}
                data={[
                  { label: "Enter value", value: "value" },
                  { label: "Upload from file", value: "file" },
                ]}
              />

              {inputMode === "value" && (
                <div className="flex flex-col gap-1.5">
                  {revealed ? (
                    <Textarea
                      label="Value"
                      description="Value is masked by default. Click the eye icon to reveal."
                      placeholder="TOP SECRET"
                      required
                      autosize
                      minRows={3}
                      maxRows={8}
                      value={currentValue}
                      onChange={(e) => {
                        const next = WsPB.UserSecret.clone(req);
                        next.data!.type = {
                          oneofKind: "value",
                          value: e.currentTarget.value,
                        };
                        setReq(next);
                      }}
                    />
                  ) : (
                    <PasswordInput
                      label="Value"
                      description="Value is masked by default. Click the eye icon to reveal."
                      placeholder="TOP SECRET"
                      required
                      autoComplete="new-password"
                      value={currentValue}
                      onChange={(e) => {
                        const next = WsPB.UserSecret.clone(req);
                        next.data!.type = {
                          oneofKind: "value",
                          value: e.currentTarget.value,
                        };
                        setReq(next);
                      }}
                      visibilityToggleIcon={({ reveal }) =>
                        reveal ? <EyeOff size={14} /> : <Eye size={14} />
                      }
                      onVisibilityChange={setRevealed}
                    />
                  )}
                </div>
              )}

              {inputMode === "file" && (
                <div className="flex flex-col gap-2">
                  <input
                    ref={fileInputRef}
                    type="file"
                    className="hidden"
                    onChange={handleFileChange}
                  />
                  <Button
                    variant="default"
                    size="sm"
                    leftSection={<Upload size={13} strokeWidth={2.5} />}
                    onClick={() => fileInputRef.current?.click()}
                    styles={{ root: { width: "fit-content" } }}
                  >
                    {fileName ? "Replace file" : "Choose file"}
                  </Button>
                  {fileName && (
                    <div className="flex items-center gap-2">
                      <FileText
                        size={12}
                        className="text-slate-400 shrink-0"
                        strokeWidth={2.5}
                      />
                      <span className="text-[0.72rem] font-semibold text-slate-500">
                        {fileName}
                      </span>
                      {currentValue && (
                        <span className="text-[0.7rem] font-semibold text-emerald-600">
                          ✓ {currentValue.length} bytes loaded
                        </span>
                      )}
                    </div>
                  )}
                  <span className="text-[0.7rem] font-semibold text-slate-400">
                    File contents will be stored as the secret value.
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <div className="flex items-center justify-between pt-4 border-t border-slate-200">
        {mutation.isError && (
          <span className="text-[0.72rem] font-semibold text-red-600">
            Creation failed — check the form and try again.
          </span>
        )}
        <div className="flex-1" />
        <div className="flex items-center gap-2">
          <Button
            variant="default"
            size="sm"
            leftSection={<X size={13} strokeWidth={2.5} />}
            disabled={mutation.isPending}
            onClick={() => navigate(-1)}
          >
            Cancel
          </Button>
          <Button
            variant="filled"
            color="dark"
            size="sm"
            leftSection={
              mutation.isPending ? (
                <Loader2 size={13} strokeWidth={2.5} className="animate-spin" />
              ) : (
                <Plus size={13} strokeWidth={2.5} />
              )
            }
            disabled={mutation.isPending || !currentValue}
            loading={mutation.isPending}
            onClick={() => mutation.mutate()}
          >
            {mutation.isPending ? "Creating…" : "Create UserSecret"}
          </Button>
        </div>
      </div>
    </div>
  );
};

export default CreateUserSecret;
