import * as UserPB from "@octelium/apis/main/userv1";

interface Settings {
  terminalFullscreen: boolean;
  terminalFontSize: number;
  itemsPerPage: number;
  navCollapsed: boolean;
  status?: UserPB.GetStatusResponse;
}

export default Settings;
