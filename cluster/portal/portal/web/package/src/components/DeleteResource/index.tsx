import { MdDelete } from "react-icons/md";

import { Button, Modal } from "@mantine/core";
import { useDisclosure } from "@mantine/hooks";

const DeleteResource = (props: {
  onDelete: () => void;
  btnSize?: "xs" | "small";
}) => {
  const [opened, { open, close }] = useDisclosure(false);
  return (
    <>
      <Button
        size={props.btnSize ?? "small"}
        variant={"outline"}
        color={`red`}
        onClick={open}
      >
        <span className="mr-1">Delete</span>
        <MdDelete />
      </Button>

      <Modal opened={opened} onClose={close} centered>
        <div className="font-bold text-xl mb-4">
          {`Are you sure that you want to delete this resource?`}
        </div>

        <div className="mt-4 flex justify-end items-center">
          <Button variant="outline" onClick={close}>
            Cancel
          </Button>
          <Button
            className="ml-4"
            // loading={mutationLogout.isPending}
            onClick={() => {
              props.onDelete();
            }}
            autoFocus
          >
            Yes, Delete
          </Button>
        </div>
      </Modal>
    </>
  );
};

export default DeleteResource;
