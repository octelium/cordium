export default () => {
  return (
    <>
      <div className="w-full mb-6 pt-8">
        <div className="w-full px-2">
          <div className="w-full flex items-center justify-center">
            <a href="https://octelium.com" target="_blank noreferrer noopener">
              <span className="text-sm font-semibold transition-all duration-300 text-gray-400 sm:text-center hover:text-gray-600">
                © {new Date().getFullYear()}{" "}
                <span className="ml-1">octelium.com</span>
              </span>
            </a>
          </div>
        </div>
      </div>
    </>
  );
};
