_memo_options() {
    local -a __memo_options
    __memo_options=(
        '--help:show help'
        '-h:show help'
        '--version:print the version'
        '-v:print the version'
     )
    _describe -t option "option" __memo_options
}

_memo_sub_commands() {
    local -a __memo_sub_commands
    __memo_sub_commands=(
     'new:create memo'
     'n:create memo'
     'list:list memo'
     'l:list memo'
     'edit:edit memo'
     'e:edit memo'
     'delete:delete memo'
     'd:delete memo'
     'grep:grep memo'
     'g:grep memo'
     'config:configure'
     'c:configure'
     'serve:start http server'
     's:start http server'
     'help:Shows a list of commands or help for one command'
     'h:Shows a list of commands or help for one command'
     )
    _describe -t command "command" __memo_sub_commands
}

_memo_list() {
    local -a __memo_list
    PRE_IFS=$IFS
    IFS=$'\n'
    __memo_list=($(memo list))
    IFS=$PRE_IFS
    _describe -t memo "memo" __memo_list
}

_memo () {
    local state line

    _arguments \
        '1: :->objects' \
        '*: :->args' \
        && ret=0

    case $state in
        objects)
            case $line[1] in
                -*)
                    _memo_options
                    ;;
                *)
                    _memo_sub_commands
                    ;;
            esac
            ;;
        args)
            last_arg="${line[${#line[@]}-1]}"

            case $last_arg in
                edit|e|delete|d)
                    _memo_list
                    ;;
                *)
                    ;;
            esac
            ;;
        *)
            _files
            ;;
    esac
}
compdef _memo memo
